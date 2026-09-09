#!/usr/bin/env python3
"""Verify go get and go install from empty module caches, with no desktop SDK."""
from __future__ import annotations
import argparse
import json
import os
import shutil
import subprocess
import tempfile
import zipfile
from pathlib import Path
from build_sdk import add_zip_file, copy_go, go_proxy
from runtime_assets import current_platform, digest, PLATFORMS
from verify_distributions import isolated, run, check_text
from version import ROOT, VERSION, TAG, RUNTIME_VERSION

APPLICATION = '''package main
import("context";"fmt";"os";oneocr "github.com/shiyori/oneocr-native")
func main(){
 source:="";if len(os.Args)>2{source=os.Args[2]}
 if _,err:=oneocr.Install(oneocr.InstallOptions{SourceDirectory:source,Offline:true});err!=nil{panic(err)}
 e,err:=oneocr.Open(oneocr.Config{});if err!=nil{panic(err)};defer e.Close()
 input:=oneocr.FromFile(os.Args[1]);ctx:=context.Background()
 r,err:=e.Recognize(ctx,input,oneocr.Options{});if err!=nil{panic(err)}
 if d,err:=e.Detect(ctx,input);err!=nil||len(d.Regions)==0{panic("detect failed")}
 if line,err:=e.RecognizeLine(ctx,input,oneocr.Options{});err!=nil||line.Text==""{panic("recognize line failed")}
 fmt.Println(r.Text)
}
'''


def prepare_proxy(directory: Path):
    source = directory / "module"
    copy_go(source)
    proxy = directory / "proxy"
    go_proxy(source, proxy)
    metadata = proxy / "github.com/shiyori/oneocr-native/@v"
    metadata.mkdir(parents=True)
    shutil.copy2(source / "go.mod", metadata / f"{TAG}.mod")
    stamp = subprocess.check_output(["git", "show", "-s", "--format=%cI", "HEAD"], cwd=ROOT, text=True).strip()
    (metadata / f"{TAG}.info").write_text(json.dumps({"Version":TAG, "Time":stamp}), encoding="utf-8", newline="\n")
    (metadata / "list").write_text(TAG + "\n", encoding="utf-8", newline="\n")
    archive = metadata / f"{TAG}.zip"
    # A test-only module proxy packages the exact production source files.
    # GOSUMDB is disabled only in these isolated pre-tag verification processes.
    with zipfile.ZipFile(archive, "w", zipfile.ZIP_DEFLATED) as zipped:
        for path in sorted(source.rglob("*")):
            if path.is_file():
                add_zip_file(zipped, path, f"github.com/shiyori/oneocr-native@{TAG}/" + path.relative_to(source).as_posix())
    return proxy, digest(archive)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--dist", type=Path, required=True)
    parser.add_argument("--runtime-cache", type=Path, required=True)
    parser.add_argument("--report", type=Path, required=True)
    args = parser.parse_args()
    dist = args.dist.resolve()
    target = current_platform()
    with tempfile.TemporaryDirectory(prefix="oneocr-go-module-consumer-") as temporary:
        root = Path(temporary)
        proxy, checksum = prepare_proxy(root)
        resources = root / "resources"
        resources.mkdir()
        for name in ("release-manifest.json", "SHA256SUMS", f"oneocr-model-cjk-en-{VERSION}.ocrpack"):
            shutil.copy2(dist / name, resources / name)
        if target.startswith("linux"):
            name = f"oneocr-runtime-{RUNTIME_VERSION}-{target}.zip"
            shutil.copy2(dist / name, resources / name)
        else:
            name = f"onnxruntime-{PLATFORMS[target]}-{RUNTIME_VERSION}" + (".zip" if target.startswith("windows") else ".tgz")
            shutil.copy2(args.runtime_cache / name, resources / name)
        app = root / "application"
        app.mkdir()
        (app / "main.go").write_text(APPLICATION, encoding="utf-8")
        image = app / "中文 图片.png"
        shutil.copy2(ROOT / "testdata/CJK.png", image)
        env = isolated(root / "library-environment")
        env.update(GOPROXY=proxy.as_uri(), GOSUMDB="off", CGO_ENABLED="1")
        run("go", "mod", "init", "example.com/application", cwd=app, env=env)
        run("go", "get", "github.com/shiyori/oneocr-native@" + TAG, cwd=app, env=env)
        check_text(run("go", "run", ".", image, resources, cwd=app, env=env).stdout)
        check_text(run("go", "run", ".", image, cwd=app, env=env).stdout)
        modules = run("go", "list", "-m", "all", cwd=app, env=env).stdout
        if "onnxruntime_go" in modules or "replace" in (app / "go.mod").read_text():
            raise RuntimeError("Go integration altered bindings or used a local replacement")
        cli_env = isolated(root / "command-environment")
        cli_env.update(GOPROXY=proxy.as_uri(), GOSUMDB="off", CGO_ENABLED="1", GOBIN=str(root / "command-bin"))
        run("go", "install", "github.com/shiyori/oneocr-native/cmd/oneocr@" + TAG, cwd=root, env=cli_env)
        cli = root / "command-bin" / ("oneocr.exe" if os.name == "nt" else "oneocr")
        run(cli, "install", "--source", resources, "--offline", cwd=app, env=cli_env)
        check_text(run(cli, "recognize", image, cwd=app, env=cli_env).stdout)
        run(cli, "install", "--offline", cwd=app, env=cli_env)
        report = {"version":VERSION, "platform":target, "source_commit":subprocess.check_output(["git", "rev-parse", "HEAD"], cwd=ROOT, text=True).strip(),
                  "test_module_sha256":checksum, "go_get":True, "library_install":True, "go_install":True,
                  "cli_install":True, "empty_module_caches":True, "no_sdk":True, "no_replace":True, "host_binding_unchanged":True, "passed":True}
    args.report.parent.mkdir(parents=True, exist_ok=True)
    args.report.write_text(json.dumps(report, indent=2) + "\n", encoding="utf-8")
    print(json.dumps(report))

if __name__ == "__main__":
    main()

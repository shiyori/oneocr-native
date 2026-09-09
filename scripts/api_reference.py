#!/usr/bin/env python3
"""Compare the new input API against the pre-refactor CPU implementation."""
from __future__ import annotations
import argparse
import json
import os
import shutil
import subprocess
import tarfile
import tempfile
from pathlib import Path
from version import ROOT

BASELINE = "1075cd8"


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--runtime",type=Path,required=True)
    parser.add_argument("--output",type=Path,required=True)
    parser.add_argument("--baseline-cli",type=Path)
    args=parser.parse_args()
    args.output=args.output.resolve();args.output.mkdir(parents=True,exist_ok=True)
    environment=dict(os.environ,ONEOCR_RUNTIME=str(args.runtime.resolve()),ONEOCR_EXPORT_REFERENCE=str(args.output))
    subprocess.run(["go","test","-run","^TestExportReferenceCrops$","-count=1", "./internal/engine"],cwd=ROOT,env=environment,check=True)
    environment.pop("ONEOCR_EXPORT_REFERENCE")
    with tempfile.TemporaryDirectory(prefix="oneocr-original-cpu-") as temporary:
        temporary=Path(temporary)
        cli=args.baseline_cli
        if cli is None:
            snapshot=temporary / "source";snapshot.mkdir()
            archive=temporary / "source.tar"
            with archive.open("wb") as output:
                subprocess.run(["git","archive",BASELINE],cwd=ROOT,stdout=output,check=True)
            with tarfile.open(archive) as source: source.extractall(snapshot,filter="data")
            # Only adapt the original runtime binding's capability request. The
            # baseline's model graph, preprocessing and OCR algorithms stay intact.
            runtime=snapshot / "runtime.go"
            runtime.write_text(runtime.read_text().replace("minor < 29","minor < 26"),encoding="utf-8")
            metadata=json.loads(subprocess.check_output(["go","mod","download","-json","github.com/yalue/onnxruntime_go@v1.35.0"],cwd=snapshot,text=True))
            binding=temporary / "original-binding"
            shutil.copytree(metadata["Dir"],binding)
            header=binding / "onnxruntime_c_api.h"
            header.chmod(0o644)
            header.write_text(header.read_text().replace("#define ORT_API_VERSION 29","#define ORT_API_VERSION 26"),encoding="utf-8")
            subprocess.run(["go","mod","edit",f"-replace=github.com/yalue/onnxruntime_go={binding}"],cwd=snapshot,check=True)
            cli=temporary / ("reference.exe" if os.name=="nt" else "reference")
            subprocess.run(["go","build","-o",str(cli),"./cmd/oneocr"],cwd=snapshot,check=True)
        cases=json.loads((args.output / "cases.json").read_text(encoding="utf-8"))
        for index,case in enumerate(cases):
            for command,key,error_key,image in (
                ("recognize","recognize","recognize_error",ROOT / case["file"]),
                ("detect","detect","detect_error",ROOT / case["file"]),
                ("recognize-line","recognize_line","line_error",args.output / case["line_file"]),
            ):
                result=subprocess.run([str(cli),command,"--model",str(ROOT / "models/oneocr-cjk-en.ocrpack"),"--runtime",str(args.runtime.resolve()),"--threads","1","--format","json",str(image)],cwd=temporary,env=environment,text=True,stdout=subprocess.PIPE,stderr=subprocess.PIPE)
                if result.returncode:
                    error=result.stderr.strip()
                    if not error.startswith("oneocr: "):
                        raise RuntimeError(f"baseline failed unexpectedly: {error}")
                    case[error_key]=error.removeprefix("oneocr: ")
                else:
                    case[key]=json.loads(result.stdout)
            if (index+1)%6==0: print(f"reference: {index+1}/{len(cases)}",flush=True)
        (args.output / "reference.json").write_text(json.dumps(cases,ensure_ascii=False,indent=2)+"\n",encoding="utf-8")
    environment["ONEOCR_REFERENCE"]=str(args.output)
    result=subprocess.run(["go","test","-run","^TestAPIReference$","-count=1", "./internal/engine"],cwd=ROOT,env=environment,text=True,stdout=subprocess.PIPE,stderr=subprocess.STDOUT)
    (args.output / "comparison.log").write_text(result.stdout,encoding="utf-8")
    if result.returncode: raise RuntimeError(result.stdout[-12000:])
    print(json.dumps({"baseline_commit":BASELINE,"fixtures":len(cases),"operations":3,"input_forms":4,"comparisons":len(cases)*3*4,"passed":True}))
if __name__ == "__main__":
    main()

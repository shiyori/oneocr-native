#!/usr/bin/env python3
"""Build an independent Android consumer and run OCR on a connected device."""
from __future__ import annotations
import argparse
import json
import os
import shutil
import subprocess
import tempfile
import time
import zipfile
from pathlib import Path
from version import ROOT
from runtime_assets import digest

ACTIVITY = r'''package dev.oneocr.releaseverification;
import android.app.Activity;
import android.os.Bundle;
import android.graphics.Bitmap;
import android.graphics.BitmapFactory;
import android.graphics.Canvas;
import android.graphics.ColorSpace;
import android.graphics.ImageDecoder;
import dev.oneocr.OneOcr;
import java.io.*;
import java.nio.ByteBuffer;
import org.json.*;

public final class MainActivity extends Activity {
    private static void require(boolean condition, String message) {
        if (!condition) throw new AssertionError(message);
    }
    private static JSONObject stable(String value) throws Exception {
        JSONObject object = new JSONObject(value); object.remove("elapsed_seconds"); return object;
    }
    private static void same(String a, String b, String label) throws Exception {
        require(stable(a).toString().equals(stable(b).toString()), label + " output mismatch");
    }
    private byte[] asset(String name) throws IOException {
        try (InputStream in = getAssets().open(name); ByteArrayOutputStream out = new ByteArrayOutputStream()) {
            byte[] buffer = new byte[65536]; int count;
            while ((count = in.read(buffer)) != -1) out.write(buffer, 0, count);
            return out.toByteArray();
        }
    }
    private static byte[] png(Bitmap bitmap) {
        ByteArrayOutputStream out = new ByteArrayOutputStream();
        require(bitmap.compress(Bitmap.CompressFormat.PNG, 100, out), "reference PNG encode failed");
        return out.toByteArray();
    }
    private JSONObject verify() throws Exception {
        byte[] data = asset("CJK.png");
        File image = new File(getFilesDir(), "中文 图片.png");
        try (FileOutputStream out = new FileOutputStream(image)) { out.write(data); }
        Bitmap bitmap = BitmapFactory.decodeByteArray(data, 0, data.length);
        require(bitmap != null, "decode failed");
        HostProbe.open(this);
        OneOcr.Options config = new OneOcr.Options(); config.threads = 1;
        OneOcr engine = OneOcr.open(this, config);
        String reference = engine.recognize(OneOcr.Input.fromEncoded(data));
        require(new JSONObject(reference).getString("text").equals("你好世界 日本語テスト 한국어 123"), reference);
        same(reference, engine.recognize(OneOcr.Input.fromFile(image.getAbsolutePath())), "file");
        same(reference, engine.recognize(OneOcr.Input.fromBitmap(bitmap)), "bitmap");
        int w = bitmap.getWidth(), h = bitmap.getHeight(), stride = w * 4 + 16;
        int[] colors = new int[w*h]; bitmap.getPixels(colors, 0, w, 0, 0, w, h);
        byte[] rgba = new byte[stride*h];
        for (int y=0;y<h;y++) for (int x=0;x<w;x++) {
            int c=colors[y*w+x], i=y*stride+x*4;
            rgba[i]=(byte)(c>>16); rgba[i+1]=(byte)(c>>8); rgba[i+2]=(byte)c; rgba[i+3]=(byte)(c>>>24);
        }
        same(reference, engine.recognize(OneOcr.Input.fromPixels(rgba,w,h,stride,OneOcr.PixelFormat.RGBA,false)), "padded RGBA");
        String detection = engine.detect(OneOcr.Input.fromEncoded(data));
        same(detection, engine.detect(OneOcr.Input.fromBitmap(bitmap)), "detect bitmap");
        JSONArray regions = new JSONObject(detection).getJSONArray("regions");
        require(regions.length()>0, "no detected regions");
        JSONArray quad = regions.getJSONObject(0).getJSONArray("quad");
        double minX=w,minY=h,maxX=0,maxY=0;
        for(int i=0;i<4;i++){ JSONArray point=quad.getJSONArray(i); minX=Math.min(minX,point.getDouble(0));maxX=Math.max(maxX,point.getDouble(0));minY=Math.min(minY,point.getDouble(1));maxY=Math.max(maxY,point.getDouble(1)); }
        int left=Math.max(0,(int)Math.floor(minX)-2), top=Math.max(0,(int)Math.floor(minY)-2);
        int right=Math.min(w,(int)Math.ceil(maxX)+2), bottom=Math.min(h,(int)Math.ceil(maxY)+2);
        Bitmap line=Bitmap.createBitmap(bitmap,left,top,right-left,bottom-top);
        OneOcr.CallOptions call = new OneOcr.CallOptions(); call.script="CJK"; call.timeoutMs=30000;
        String lineResult=engine.recognizeLine(OneOcr.Input.fromBitmap(line),call);
        same(lineResult,engine.recognizeLine(OneOcr.Input.fromEncoded(png(line)),call),"line bitmap");
        require(!new JSONObject(lineResult).getString("text").isEmpty(),"empty cropped-line result");
        Bitmap alpha=Bitmap.createBitmap(w,h,Bitmap.Config.ARGB_8888);
        Canvas canvas=new Canvas(alpha); android.graphics.Paint paint=new android.graphics.Paint();paint.setAlpha(192);canvas.drawBitmap(bitmap,0,0,paint);
        // Compare the stored premultiplied pixels composited onto white.
        // A transparent PNG roundtrip first unpremultiplies and re-quantizes.
        Bitmap alphaWhite=Bitmap.createBitmap(w,h,Bitmap.Config.ARGB_8888);
        Canvas whiteCanvas=new Canvas(alphaWhite);whiteCanvas.drawColor(android.graphics.Color.WHITE);whiteCanvas.drawBitmap(alpha,0,0,null);
        same(engine.recognize(OneOcr.Input.fromEncoded(png(alphaWhite))),engine.recognize(OneOcr.Input.fromBitmap(alpha)),"premultiplied alpha");
        int hardware=0;
        if(android.os.Build.VERSION.SDK_INT>=28){
            Bitmap hardwareBitmap=ImageDecoder.decodeBitmap(ImageDecoder.createSource(ByteBuffer.wrap(data)),(decoder,info,source)->decoder.setAllocator(ImageDecoder.ALLOCATOR_HARDWARE));
            same(reference,engine.recognize(OneOcr.Input.fromBitmap(hardwareBitmap)),"hardware bitmap");hardwareBitmap.recycle();hardware=1;
        }
        Bitmap p3=Bitmap.createBitmap(w,h,Bitmap.Config.ARGB_8888,true,ColorSpace.get(ColorSpace.Named.DISPLAY_P3));
        new Canvas(p3).drawBitmap(bitmap,0,0,null);
        Bitmap normalized=Bitmap.createBitmap(w,h,Bitmap.Config.ARGB_8888);
        new Canvas(normalized).drawBitmap(p3,0,0,null);
        same(engine.recognize(OneOcr.Input.fromBitmap(normalized)),engine.recognize(OneOcr.Input.fromBitmap(p3)),"sRGB normalization");
        boolean invalid=false;
        try{engine.detect(OneOcr.Input.fromPixels(new byte[1],Integer.MAX_VALUE,2,4,OneOcr.PixelFormat.RGBA,false));}catch(RuntimeException expected){invalid=true;}
        require(invalid,"accepted invalid dimensions");
        OneOcr.CallOptions timeout=new OneOcr.CallOptions();timeout.timeoutMs=-1;invalid=false;
        try{engine.recognize(OneOcr.Input.fromEncoded(data),timeout);}catch(RuntimeException expected){invalid=true;}
        require(invalid,"accepted negative timeout");
        engine.warmup(30000);
        JSONObject diagnostics=new JSONObject(engine.diagnostics());
        engine.close();engine.close();
        invalid=false;try{engine.recognize(OneOcr.Input.fromEncoded(data));}catch(IllegalStateException expected){invalid=true;}
        require(invalid,"accepted call after close");
        HostProbe.verifyAndClose();
        bitmap.recycle();line.recycle();alpha.recycle();alphaWhite.recycle();p3.recycle();normalized.recycle();
        JSONObject result=new JSONObject();result.put("ok",true);result.put("runtime",diagnostics.getString("runtime_version"));result.put("abi",android.os.Build.SUPPORTED_ABIS[0]);result.put("hardware_test",hardware);result.put("text",new JSONObject(reference).getString("text"));result.put("line_text",new JSONObject(lineResult).getString("text"));result.put("input_forms",new JSONArray(new String[]{"encoded","file","bitmap","RGBA-stride","premultiplied","P3"}));return result;
    }
    @Override public void onCreate(Bundle saved){super.onCreate(saved);new Thread(()->{
        JSONObject result;
        try{result=verify();}catch(Throwable error){result=new JSONObject();try{result.put("ok",false);result.put("error",android.util.Log.getStackTraceString(error));}catch(Exception ignored){}}
        try(FileOutputStream output=new FileOutputStream(new File(getFilesDir(),"verification.json"))){output.write(result.toString().getBytes(java.nio.charset.StandardCharsets.UTF_8));}catch(Exception error){android.util.Log.e("OneOCRVerify","write failed",error);}
    }).start();}
}
'''
STUB = '''package dev.oneocr.releaseverification; import android.content.Context;
final class HostProbe { static void open(Context context) {} static void verifyAndClose() {} }
'''
HOST = '''package dev.oneocr.releaseverification;
import android.content.Context;
import ai.onnxruntime.*;
import java.util.*;
final class HostProbe {
    static OrtEnvironment environment; static OrtSession session;
    static void open(Context context) throws Exception {
        environment=OrtEnvironment.getEnvironment();
        byte[] model;try(java.io.InputStream in=context.getAssets().open("identity.onnx")){java.io.ByteArrayOutputStream out=new java.io.ByteArrayOutputStream();byte[] b=new byte[4096];int n;while((n=in.read(b))!=-1)out.write(b,0,n);model=out.toByteArray();}
        try(OrtSession.SessionOptions options=new OrtSession.SessionOptions()){session=environment.createSession(model,options);}
        check();
    }
    static void check() throws Exception {
        try(OnnxTensor input=OnnxTensor.createTensor(environment,java.nio.FloatBuffer.wrap(new float[]{42}),new long[]{1});OrtSession.Result result=session.run(Collections.singletonMap("x",input))){
            float[] output=(float[])result.get(0).getValue();if(output[0]!=42)throw new AssertionError("host session changed");
        }
    }
    static void verifyAndClose() throws Exception {check();session.close();environment.close();}
}
'''


def run(command, *, cwd=None, check=True):
    result = subprocess.run([str(p) for p in command], cwd=cwd, text=True, stdout=subprocess.PIPE, stderr=subprocess.STDOUT)
    if check and result.returncode:
        raise RuntimeError(result.stdout[-12000:])
    return result


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--aar", type=Path, required=True)
    parser.add_argument("--sdk", type=Path, default=os.environ.get("ANDROID_HOME"), required=os.environ.get("ANDROID_HOME") is None)
    parser.add_argument("--gradle", default="gradle")
    parser.add_argument("--serial")
    parser.add_argument("--abi", choices=("arm64-v8a","x86_64"), required=True)
    parser.add_argument("--host-version")
    parser.add_argument("--model", type=Path, default=ROOT / "models/oneocr-cjk-en.ocrpack")
    parser.add_argument("--report", type=Path, required=True)
    args = parser.parse_args()
    adb = [args.sdk / "platform-tools" / ("adb.exe" if os.name == "nt" else "adb")]
    if args.serial: adb += ["-s", args.serial]
    abi = run(adb + ["shell","getprop","ro.product.cpu.abi"]).stdout.strip()
    if abi != args.abi:
        raise RuntimeError(f"device ABI {abi} does not match requested {args.abi}")
    with tempfile.TemporaryDirectory(prefix="oneocr-android-consumer-") as temporary:
        root = Path(temporary)
        (root / "settings.gradle.kts").write_text('pluginManagement { repositories { google(); mavenCentral(); gradlePluginPortal() } }\ndependencyResolutionManagement { repositories { google(); mavenCentral() } }\nrootProject.name="oneocr-consumer"\ninclude(":app")\n',encoding="utf-8")
        (root / "build.gradle.kts").write_text('plugins { id("com.android.application") version "9.0.1" apply false }\n',encoding="utf-8")
        (root / "gradle.properties").write_text('org.gradle.jvmargs=-Xmx2048m -Dfile.encoding=UTF-8\nandroid.useAndroidX=true\n',encoding="utf-8")
        (root / "local.properties").write_text('sdk.dir=' + str(args.sdk.resolve()).replace('\\','\\\\').replace(':','\\:') + '\n',encoding="utf-8")
        app = root / "app"
        (app / "libs").mkdir(parents=True)
        shutil.copy2(args.aar,app / "libs/oneocr.aar")
        dependency = f'implementation("com.microsoft.onnxruntime:onnxruntime-android:{args.host_version}")' if args.host_version else ''
        (app / "build.gradle.kts").write_text('plugins { id("com.android.application") }\nandroid { namespace="dev.oneocr.releaseverification"; compileSdk=36\n defaultConfig { applicationId="dev.oneocr.releaseverification"; minSdk=26; targetSdk=36; versionCode=1; versionName="1"; ndk { abiFilters += "'+args.abi+'" } }\n compileOptions { sourceCompatibility=JavaVersion.VERSION_17; targetCompatibility=JavaVersion.VERSION_17 }\n}\ndependencies { implementation(files("libs/oneocr.aar")); '+dependency+' }\n',encoding="utf-8")
        main = app / "src/main"
        java = main / "java/dev/oneocr/releaseverification"
        java.mkdir(parents=True)
        (java / "MainActivity.java").write_text(ACTIVITY,encoding="utf-8")
        (java / "HostProbe.java").write_text(HOST if args.host_version else STUB,encoding="utf-8")
        (main / "AndroidManifest.xml").write_text('<manifest xmlns:android="http://schemas.android.com/apk/res/android"><application android:label="OneOCR verification" android:theme="@android:style/Theme.Material.Light.NoActionBar"><activity android:name=".MainActivity" android:exported="true" /></application></manifest>\n',encoding="utf-8")
        assets = main / "assets"
        assets.mkdir()
        shutil.copy2(ROOT / "testdata/CJK.png",assets / "CJK.png")
        if args.host_version:
            shutil.copy2(args.model,assets / "oneocr-cjk-en.ocrpack")
            shutil.copy2(ROOT / "testdata/identity.onnx",assets / "identity.onnx")
        built = run([args.gradle,"--no-daemon",":app:assembleDebug"],cwd=root)
        args.report.parent.mkdir(parents=True,exist_ok=True)
        args.report.with_suffix(".build.log").write_text(built.stdout,encoding="utf-8")
        package="dev.oneocr.releaseverification"
        run(adb+["shell","am","force-stop",package],check=False)
        run(adb+["install","-r",app / "build/outputs/apk/debug/app-debug.apk"])
        run(adb+["shell","run-as",package,"rm","-f","files/verification.json"],check=False)
        run(adb+["shell","am","start","-W","-n",package+"/.MainActivity"])
        deadline=time.monotonic()+240
        result=None
        while time.monotonic()<deadline:
            output=run(adb+["shell","run-as",package,"cat","files/verification.json"],check=False)
            if output.returncode==0:
                try: result=json.loads(output.stdout);break
                except json.JSONDecodeError: pass
            time.sleep(1)
        if result is None:
            logs=run(adb+["logcat","-d","-t","500"],check=False).stdout
            args.report.with_suffix(".logcat.log").write_text(logs,encoding="utf-8")
            raise RuntimeError("Android OCR did not finish; see logcat report")
        result["host_version"]=args.host_version
        result["aar_name"]=args.aar.name
        result["aar_sha256"]=digest(args.aar)
        args.report.write_text(json.dumps(result,ensure_ascii=False,indent=2)+"\n",encoding="utf-8")
        run(adb+["shell","am","force-stop",package],check=False)
        if not result["ok"]: raise RuntimeError(result["error"])
        print(json.dumps(result,ensure_ascii=False))
if __name__ == "__main__":
    main()

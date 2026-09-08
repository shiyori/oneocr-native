package dev.oneocr;

import android.content.Context;
import android.graphics.Bitmap;
import java.io.ByteArrayOutputStream;
import java.io.File;
import java.io.FileInputStream;
import java.io.FileOutputStream;
import java.io.IOException;
import java.io.InputStream;
import java.nio.charset.StandardCharsets;
import java.nio.file.Files;
import java.nio.file.StandardCopyOption;
import java.security.MessageDigest;
import java.security.NoSuchAlgorithmException;

/** Offline, synchronous OCR SDK. Reuse an instance on a background executor. */
public final class OneOcr implements AutoCloseable {
    public static final String DEFAULT_MODEL = "oneocr-cjk-en.ocrpack";
    static { System.loadLibrary("oneocr_jni"); }
    private long handle;

    /** Opens the default model from app/src/main/assets with two CPU threads. */
    public static OneOcr fromAsset(Context context) throws IOException {
        return fromAsset(context, DEFAULT_MODEL, 2);
    }

    /** Opens the default asset with the requested CPU thread count. */
    public static OneOcr fromAsset(Context context, int threads) throws IOException {
        return fromAsset(context, DEFAULT_MODEL, threads);
    }

    /** Uses the ONNX Runtime included in the AAR; model is an .ocrpack or legacy directory. */
    public OneOcr(String model, int threads) { this(model, "", threads); }

    /** An explicit runtime path remains supported for applications managing their own runtime. */
    public OneOcr(String model, String runtimeLibrary, int threads) {
        if (model == null || runtimeLibrary == null) throw new NullPointerException("paths");
        handle = nativeOpen(utf8(model), utf8(runtimeLibrary), threads);
    }

    /** Streams one asset into a content-addressed private file, then opens it.
     * No resource directory is extracted and the complete package is never held in a byte[]. */
    public static OneOcr fromAsset(Context context, String assetName, int threads) throws IOException {
        if (context == null || assetName == null) throw new NullPointerException("context/assetName");
        File directory = new File(context.getFilesDir(), "oneocr/models");
        if (!directory.isDirectory() && !directory.mkdirs()) throw new IOException("cannot create model directory");
        File temporary = File.createTempFile(".import-", ".ocrpack", directory);
        try {
            MessageDigest digest = newDigest();
            long count = 0;
            try (InputStream input = context.getAssets().open(assetName);
                 FileOutputStream output = new FileOutputStream(temporary)) {
                byte[] buffer = new byte[64 * 1024];
                int size;
                while ((size = input.read(buffer)) != -1) {
                    count += size;
                    if (count > 2L * 1024 * 1024 * 1024) throw new IOException("model exceeds 2 GiB");
                    digest.update(buffer, 0, size);
                    output.write(buffer, 0, size);
                }
                output.getFD().sync();
            }
            String hash = hex(digest.digest());
            File destination = new File(directory, hash + ".ocrpack");
            if (!destination.isFile() || destination.length() != count || !hash.equals(fileHash(destination))) {
                Files.move(temporary.toPath(), destination.toPath(),
                        StandardCopyOption.ATOMIC_MOVE, StandardCopyOption.REPLACE_EXISTING);
            }
            return new OneOcr(destination.getAbsolutePath(), threads);
        } finally {
            if (temporary.exists()) temporary.delete();
        }
    }
    private static MessageDigest newDigest() {
        try { return MessageDigest.getInstance("SHA-256"); }
        catch (NoSuchAlgorithmException error) { throw new AssertionError(error); }
    }
    private static String fileHash(File file) throws IOException {
        MessageDigest digest = newDigest();
        try (InputStream input = new FileInputStream(file)) {
            byte[] buffer = new byte[64 * 1024];
            int count;
            while ((count = input.read(buffer)) != -1) digest.update(buffer, 0, count);
        }
        return hex(digest.digest());
    }
    private static String hex(byte[] bytes) {
        char[] chars = "0123456789abcdef".toCharArray();
        StringBuilder result = new StringBuilder(bytes.length * 2);
        for (byte value : bytes) { result.append(chars[(value & 255) >>> 4]); result.append(chars[value & 15]); }
        return result.toString();
    }
    private static byte[] utf8(String text) {
        if (text.indexOf('\0') >= 0) throw new IllegalArgumentException("NUL in path");
        return text.getBytes(StandardCharsets.UTF_8);
    }
    public synchronized String recognize(byte[] encodedImage) {
        if (handle == 0) throw new IllegalStateException("OneOCR is closed");
        if (encodedImage == null) throw new NullPointerException("encodedImage");
        return nativeRecognize(handle, encodedImage);
    }
    public synchronized String recognize(Bitmap bitmap) {
        if (bitmap == null) throw new NullPointerException("bitmap");
        ByteArrayOutputStream bytes = new ByteArrayOutputStream();
        if (!bitmap.compress(Bitmap.CompressFormat.PNG, 100, bytes)) throw new IllegalArgumentException("cannot encode bitmap");
        return recognize(bytes.toByteArray());
    }
    /** Detects regions without running script classification or recognition. */
    public synchronized String detect(byte[] encodedImage) {
        if (handle == 0) throw new IllegalStateException("OneOCR is closed");
        if (encodedImage == null) throw new NullPointerException("encodedImage");
        return nativeStage(handle, encodedImage, null, true);
    }
    /** Recognizes a cropped horizontal line. Null/empty script auto-classifies
     * and corrects 180-degree rotation; an explicit script assumes upright input. */
    public synchronized String recognizeLine(byte[] encodedImage, String script) {
        if (handle == 0) throw new IllegalStateException("OneOCR is closed");
        if (encodedImage == null) throw new NullPointerException("encodedImage");
        return nativeStage(handle, encodedImage, script == null ? null : utf8(script), false);
    }
    public synchronized String recognizeLine(byte[] encodedImage) {
        return recognizeLine(encodedImage, null);
    }
    @Override public synchronized void close() {
        long value = handle;
        handle = 0;
        if (value != 0) nativeClose(value);
    }
    private static native long nativeOpen(byte[] bundle, byte[] runtime, int threads);
    private static native String nativeRecognize(long handle, byte[] image);
    private static native String nativeStage(long handle, byte[] image, byte[] script, boolean detect);
    private static native void nativeClose(long handle);
}

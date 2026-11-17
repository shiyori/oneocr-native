package dev.oneocr;

import android.content.Context;
import android.graphics.Bitmap;
import android.graphics.Canvas;
import android.graphics.ColorSpace;
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
import java.util.concurrent.ExecutorService;
import java.util.concurrent.Executors;
import java.util.concurrent.Future;
import java.util.concurrent.locks.ReentrantReadWriteLock;

/** Offline, synchronous OCR SDK. Reuse an instance on a background executor. */
public final class OneOcr implements AutoCloseable {
    public static final String DEFAULT_MODEL = "oneocr-cjk-en.ocrpack";
    static { System.loadLibrary("oneocr_jni"); }
    private long handle;
    private final ReentrantReadWriteLock lifecycle = new ReentrantReadWriteLock();
    private final int maxInFlight;

    public static final class Options {
        public String modelPath = "";
        public String assetName = DEFAULT_MODEL;
        public String runtimeLibrary = "";
        public int threads = 2;
        public int maxSide = 1600;
        public int maxInFlight = 1;
        public int characterClasses = 0;
    }
    public static final class CallOptions {
        public static final int READING_ORDER_AUTO = 0;
        public static final int READING_ORDER_LTR = 1;
        public static final int READING_ORDER_RTL = 2;
        public static final int READING_ORDER_NONE = 3;
        public String script = "";
        public long timeoutMs = 0;
        /** Final line ordering policy: one of the READING_ORDER_* constants. */
        public int readingOrder = READING_ORDER_AUTO;
        /** Drop recognized lines below this score; zero disables filtering. */
        public double minLineConfidence = 0;
    }
    public enum PixelFormat { RGB(1), RGBA(2), BGRA(3), RGBX(4), BGRX(5); final int value; PixelFormat(int value) { this.value = value; } }
    /** Borrows bytes or a Bitmap until the synchronous call returns. */
    public static final class Input {
        private int kind, width, height, stride, format;
        private boolean premultiplied;
        private byte[] data, path;
        private Bitmap bitmap;
        private Input() {}
        public static Input fromFile(String path) {
            Input i = new Input(); i.kind = 1; i.path = utf8(path); return i;
        }
        public static Input fromEncoded(byte[] data) {
            if (data == null) throw new NullPointerException("data");
            Input i = new Input(); i.kind = 2; i.data = data; return i;
        }
        public static Input fromBitmap(Bitmap bitmap) {
            if (bitmap == null) throw new NullPointerException("bitmap");
            Input i = new Input(); i.kind = 3; i.bitmap = bitmap; return i;
        }
        public static Input fromPixels(byte[] data, int width, int height, int stride, PixelFormat format, boolean premultiplied) {
            if (data == null || format == null) throw new NullPointerException("pixels/format");
            Input i = new Input(); i.kind = 3; i.data = data; i.width = width; i.height = height;
            i.stride = stride; i.format = format.value; i.premultiplied = premultiplied; return i;
        }
    }
    private OneOcr(long handle, int maxInFlight) { this.handle = handle; this.maxInFlight = Math.max(1, Math.min(4, maxInFlight)); }
    /** Opens the bundled default asset. Reuse the engine on a background executor. */
    public static OneOcr open(Context context) throws IOException { return open(context, new Options()); }
    public static OneOcr open(Context context, Options options) throws IOException {
        if (context == null || options == null) throw new NullPointerException("context/options");
        String model = options.modelPath.isEmpty() ? importAsset(context, options.assetName) : options.modelPath;
        long handle = nativeOpen(utf8(model), utf8(options.runtimeLibrary), options.threads, options.maxSide, options.maxInFlight, options.characterClasses);
        return new OneOcr(handle, options.maxInFlight);
    }
    private static String importAsset(Context context, String assetName) throws IOException {
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
            return destination.getAbsolutePath();
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
    public String recognize(Input input) { return recognize(input, new CallOptions()); }
    public String recognize(Input input, CallOptions options) { return readOperate(input, options, 0); }
    public String detect(Input input) { return detect(input, new CallOptions()); }
    public String detect(Input input, CallOptions options) { return readOperate(input, options, 1); }
    public String recognizeLine(Input input) { return recognizeLine(input, new CallOptions()); }
    public String recognizeLine(Input input, CallOptions options) { return readOperate(input, options, 2); }
    public String[] recognizeBatch(Input[] inputs, CallOptions options) { return batch(inputs, options, 0); }
    public String[] detectBatch(Input[] inputs, CallOptions options) { return batch(inputs, options, 1); }
    public String[] recognizeLineBatch(Input[] inputs, CallOptions options) { return batch(inputs, options, 2); }
    private String[] batch(Input[] inputs, CallOptions options, int operation) {
        if (inputs == null || options == null) throw new NullPointerException("inputs/options");
        ExecutorService executor = Executors.newFixedThreadPool(Math.min(maxInFlight, Math.max(1, inputs.length)));
        try {
            Future<?>[] futures = new Future<?>[inputs.length];
            String[] results = new String[inputs.length];
            for (int i = 0; i < inputs.length; i++) {
                final int index = i;
                futures[i] = executor.submit(() -> {
                    try {
                        results[index] = "{\"index\":" + index + ",\"result\":"
                            + readOperate(inputs[index], options, operation) + "}";
                    } catch (RuntimeException error) {
                        results[index] = "{\"index\":" + index + ",\"error\":{\"code\":\""
                            + batchErrorCode(error) + "\",\"message\":" + quote(error.getMessage()) + "}}";
                    }
                });
            }
            for (Future<?> future : futures) {
                try { future.get(); }
                catch (Exception error) { throw new IllegalStateException(error); }
            }
            return results;
        } finally { executor.shutdown(); }
    }
    private static String batchErrorCode(RuntimeException error) {
        String message = error.getMessage() == null ? "" : error.getMessage().toLowerCase();
        if (message.contains("closed")) return "closed";
        if (message.contains("timeout") || message.contains("deadline")) return "timeout";
        if (message.contains("invalid") || message.contains("empty input") || message.contains("unknown script")) return "invalid_input";
        if (error instanceof IllegalArgumentException || error instanceof NullPointerException) return "invalid_input";
        if (message.contains("unsupported") || message.contains("unavailable")) return "unsupported";
        return "inference";
    }
    private static String quote(String value) { if (value == null) value = "OneOCR failed"; return "\"" + value.replace("\\", "\\\\").replace("\"", "\\\"").replace("\n", "\\n") + "\""; }
    private String readOperate(Input input, CallOptions options, int operation) {
        lifecycle.readLock().lock();
        try { return operate(input, options, operation); }
        finally { lifecycle.readLock().unlock(); }
    }
    private String operate(Input input, CallOptions options, int operation) {
        if (handle == 0) throw new IllegalStateException("OneOCR is closed");
        if (input == null || options == null) throw new NullPointerException("input/options");
        Bitmap bitmap = input.bitmap;
        Bitmap copied = null;
        try {
            if (bitmap != null) {
                if (bitmap.isRecycled()) throw new IllegalArgumentException("bitmap is recycled");
                if (bitmap.getWidth() < 2 || bitmap.getHeight() < 2 || (long)bitmap.getWidth() * bitmap.getHeight() > 40_000_000)
                    throw new IllegalArgumentException("image must be 2x2 to 40 megapixels");
                if (bitmap.getConfig() == Bitmap.Config.HARDWARE) {
                    copied = bitmap.copy(Bitmap.Config.ARGB_8888, false);
                    if (copied == null) throw new IllegalArgumentException("cannot read hardware bitmap");
                    bitmap = copied;
                }
                ColorSpace colorSpace = bitmap.getColorSpace();
                if (bitmap.getConfig() != Bitmap.Config.ARGB_8888 || colorSpace == null || !colorSpace.isSrgb()) {
                    Bitmap normalized = Bitmap.createBitmap(bitmap.getWidth(), bitmap.getHeight(), Bitmap.Config.ARGB_8888);
                    new Canvas(normalized).drawBitmap(bitmap, 0, 0, null);
                    if (copied != null) copied.recycle();
                    copied = normalized; bitmap = normalized;
                }
            }
            return nativeOperate(handle, input.kind, input.path, input.data, bitmap,
                input.width, input.height, input.stride, bitmap == null ? input.format : (bitmap.hasAlpha() ? 2 : 4),
                bitmap == null ? input.premultiplied : bitmap.isPremultiplied(),
                utf8(options.script == null ? "" : options.script), options.timeoutMs,
                options.readingOrder, options.minLineConfidence, operation);
        } finally { if (copied != null) copied.recycle(); }
    }
    public void warmup(long timeoutMs) {
        lifecycle.readLock().lock();
        try { if (handle == 0) throw new IllegalStateException("OneOCR is closed"); nativeWarmup(handle, timeoutMs); }
        finally { lifecycle.readLock().unlock(); }
    }
    public String diagnostics() {
        lifecycle.readLock().lock();
        try { if (handle == 0) throw new IllegalStateException("OneOCR is closed"); return nativeDiagnostics(handle); }
        finally { lifecycle.readLock().unlock(); }
    }
    @Override public void close() {
        lifecycle.writeLock().lock();
        try { long value = handle; handle = 0; if (value != 0) nativeClose(value); }
        finally { lifecycle.writeLock().unlock(); }
    }
    private static native long nativeOpen(byte[] model, byte[] runtime, int threads, int maxSide, int maxInFlight, int characterClasses);
    private static native String nativeOperate(long handle, int kind, byte[] path, byte[] data, Bitmap bitmap,
        int width, int height, int stride, int format, boolean premultiplied, byte[] script, long timeoutMs,
        int readingOrder, double minLineConfidence, int operation);
    private static native void nativeWarmup(long handle, long timeoutMs);
    private static native String nativeDiagnostics(long handle);
    private static native void nativeClose(long handle);
}

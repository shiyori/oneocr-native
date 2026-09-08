import android.content.Context;
import dev.oneocr.OneOcr;
import java.io.IOException;

// Call from a background executor. Store oneocr-cjk-en.ocrpack in app/src/main/assets.
public final class Example {
    public static String recognize(Context context, byte[] pngOrJpeg) throws IOException {
        try (OneOcr engine = OneOcr.fromAsset(context)) {
            return engine.recognize(pngOrJpeg);
        }
    }
}

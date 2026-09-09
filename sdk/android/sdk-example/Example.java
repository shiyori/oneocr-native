import android.content.Context;
import dev.oneocr.OneOcr;
import java.io.IOException;

// Call from a background executor. The complete AAR includes the model.
public final class Example {
    public static String recognize(Context context, byte[] pngOrJpeg) throws IOException {
        try (OneOcr engine = OneOcr.open(context)) {
            return engine.recognize(OneOcr.Input.fromEncoded(pngOrJpeg));
        }
    }
}

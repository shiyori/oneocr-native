// Deterministic OCR fixtures rendered with CoreText/AppKit shaping and fallback.
// Usage: swift render_fixtures.swift /tmp/oneocr-fixtures
import AppKit
import Foundation

let destination = CommandLine.arguments.count > 1 ? CommandLine.arguments[1] : "/tmp/oneocr-fixtures"
try FileManager.default.createDirectory(atPath: destination, withIntermediateDirectories: true)
let examples: [(String, String)] = [
    ("Latin", "Hello World 123"),
    ("CJK", "你好世界 日本語テスト 한국어 123"),
    ("Cyrillic", "Привет мир 123"),
    ("Arabic", "مرحبا بالعالم"),
    ("Devanagari", "नमस्ते दुनिया"),
    ("Greek", "Καλημέρα κόσμε 123"),
    ("Hebrew", "שלום עולם"),
    ("Tamil", "வணக்கம் உலகம்"),
    ("Thai", "สวัสดีชาวโลก"),
    ("CJK_basic", "你好世界"),
    ("Arabic_numbers", "السعر 123.45 USD"),
    ("Hebrew_numbers", "מחיר 123.45 USD"),
]
var annotations: [[String: Any]] = []
for (script, text) in examples {
    let width = 1000, height = 160
    let bitmap = NSBitmapImageRep(bitmapDataPlanes: nil, pixelsWide: width, pixelsHigh: height,
        bitsPerSample: 8, samplesPerPixel: 4, hasAlpha: true, isPlanar: false,
        colorSpaceName: .deviceRGB, bytesPerRow: 0, bitsPerPixel: 0)!
    NSGraphicsContext.saveGraphicsState()
    NSGraphicsContext.current = NSGraphicsContext(bitmapImageRep: bitmap)
    NSColor.white.setFill()
    NSRect(x: 0, y: 0, width: width, height: height).fill()
    let attrs: [NSAttributedString.Key: Any] = [
        .font: NSFont.systemFont(ofSize: 44), .foregroundColor: NSColor.black
    ]
    (text as NSString).draw(at: NSPoint(x: 30, y: 60), withAttributes: attrs)
    NSGraphicsContext.restoreGraphicsState()
    let data = bitmap.representation(using: .png, properties: [:])!
    try data.write(to: URL(fileURLWithPath: destination).appendingPathComponent(script + ".png"))
    annotations.append(["file": script + ".png", "script": String(script.split(separator: "_")[0]), "text": text])
}
let json = try JSONSerialization.data(withJSONObject: annotations, options: [.prettyPrinted, .sortedKeys, .withoutEscapingSlashes])
try json.write(to: URL(fileURLWithPath: destination).appendingPathComponent("annotations.json"))
print(destination)

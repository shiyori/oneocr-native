import AppKit
import Foundation
let root = CommandLine.arguments[1]
try FileManager.default.createDirectory(atPath: root, withIntermediateDirectories: true)
let sets: [(String, [[String]], CGFloat)] = [
 ("mixed-720p-a", [["屏幕文字识别测试 2026", "日本語の文字認識テスト", "한국어 문자 인식 테스트", "English OCR benchmark 1280 x 720", "数字 0123456789 123.45", "中日韓 English 日本語 한국어"]], 36),
 ("mixed-720p-b", [["任务状态：运行中", "日本語テスト 2026", "中文与数字 12345", "Hello World 123"], ["한국어 테스트 2026", "Frame size 1280 x 720", "CPU / CoreML", "Latency 16.67 ms"]], 32),
 ("mixed-720p-c", [["自动化测试 / Automation test", "当前画面分辨率 1280 x 720", "読み取り結果を確認します", "화면의 문자를 인식합니다", "English text with numbers 001 002 003", "订单编号 20260908", "日本語とEnglishの混在", "한국어 English 12345", "0123456789 9876543210"]], 28)
]
var annotations: [[String:Any]] = []
for (name, columns, size) in sets {
 let bitmap = NSBitmapImageRep(bitmapDataPlanes:nil, pixelsWide:1280, pixelsHigh:720, bitsPerSample:8, samplesPerPixel:4, hasAlpha:true, isPlanar:false, colorSpaceName:.deviceRGB, bytesPerRow:0, bitsPerPixel:0)!
 NSGraphicsContext.saveGraphicsState(); NSGraphicsContext.current = NSGraphicsContext(bitmapImageRep:bitmap)
 NSColor.white.setFill(); NSRect(x:0,y:0,width:1280,height:720).fill()
 for (col, lines) in columns.enumerated() {
  let gap:CGFloat = lines.count>6 ? 65 : (columns.count==2 ? 140 : 100)
  for (row,text) in lines.enumerated() {
   let attrs:[NSAttributedString.Key:Any] = [.font:NSFont.systemFont(ofSize:size), .foregroundColor:NSColor.black]
   (text as NSString).draw(at:NSPoint(x:48+CGFloat(col)*625,y:650-CGFloat(row)*gap), withAttributes:attrs)
  }
 }
 NSGraphicsContext.restoreGraphicsState()
 let file=name+".png";try bitmap.representation(using:.png,properties:[:])!.write(to:URL(fileURLWithPath:root).appendingPathComponent(file))
 annotations.append(["file":file,"text":columns.flatMap{$0}.joined(separator:"\n"),"width":1280,"height":720,"font_size":size,"synthetic":true])
}
try JSONSerialization.data(withJSONObject:annotations,options:[.prettyPrinted,.sortedKeys,.withoutEscapingSlashes]).write(to:URL(fileURLWithPath:root).appendingPathComponent("annotations.json"))
print(root)

# ChangeLog - Novel Reader

## 2026-04-24

### Added
- **Expanded Encoding Support**: 小说导入支持 UTF-8、GBK、GB18030、Big5、UTF-16 LE/BE、Shift_JIS、EUC-JP、EUC-KR、Windows-1252、Windows-1251、ISO-8859-1，并在自动识别中统一尝试这些常见编码。
- **Expanded File Import Support**: 书架导入支持 `.txt`、`.text`、`.md`、`.markdown`、`.log`、`.html`、`.htm`、`.xhtml`、`.xml`、`.epub` 等常见小说文件类型。
- **EPUB Lightweight Parser**: 新增浏览器端 EPUB 轻量解析，读取 ZIP 中的 OPF spine 顺序并提取 XHTML/HTML 正文。

### Changed
- **Fish Mode Page Turn**: 摸鱼模式向下翻页改为基于真实文本行位置定位，若底部行被截断，下一页会从该行完整显示开始。
- **Import Copywriting**: 添加书籍弹窗从 TXT 单一导入说明升级为本地小说文件导入说明，并复用统一的编码与文件类型配置。

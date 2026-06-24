import 'package:flutter/material.dart';
import 'package:provider/provider.dart';
import '../../services/subscription_service.dart';

/// QR scanner page: scans a QR code and auto-imports subscription.
///
/// Since mobile_scanner is unavailable (no pub.dev access), this page
/// shows a camera-unavailable placeholder and prompts the user to
/// manually input their subscription link instead.
///
/// Returns `true` via Navigator.pop if import succeeded.
class QrScannerPage extends StatefulWidget {
  const QrScannerPage({super.key});

  @override
  State<QrScannerPage> createState() => _QrScannerPageState();
}

class _QrScannerPageState extends State<QrScannerPage> {
  final _linkController = TextEditingController();
  final _formKey = GlobalKey<FormState>();
  bool _isImporting = false;
  String? _error;

  @override
  void dispose() {
    _linkController.dispose();
    super.dispose();
  }

  Future<void> _importLink() async {
    if (!_formKey.currentState!.validate()) return;

    setState(() {
      _isImporting = true;
      _error = null;
    });

    final text = _linkController.text.trim();
    final subService = context.read<SubscriptionService>();
    subService.clearImport();

    bool success;
    if (text.startsWith('http://') || text.startsWith('https://')) {
      success = await subService.importFromUrl(text);
    } else {
      success = await subService.importFromLink(text);
    }

    if (!mounted) return;

    if (success) {
      ScaffoldMessenger.of(context).showSnackBar(
        const SnackBar(content: Text('导入成功')),
      );
      Navigator.of(context).pop(true);
    } else {
      setState(() {
        _error = subService.importError ?? '无法解析输入内容';
        _isImporting = false;
      });
    }
  }

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);

    return Scaffold(
      appBar: AppBar(
        title: const Text('扫描二维码'),
        leading: IconButton(
          icon: const Icon(Icons.close),
          onPressed: () => Navigator.of(context).pop(false),
        ),
      ),
      body: SingleChildScrollView(
        child: Column(
          children: [
            const SizedBox(height: 32),

            // Camera unavailable placeholder
            Container(
              margin: const EdgeInsets.symmetric(horizontal: 24),
              padding: const EdgeInsets.all(32),
              decoration: BoxDecoration(
                color: Colors.grey.withAlpha(20),
                borderRadius: BorderRadius.circular(16),
                border: Border.all(color: Colors.grey.withAlpha(60)),
              ),
              child: Column(
                children: [
                  Icon(Icons.qr_code_scanner, size: 72, color: Colors.grey[500]),
                  const SizedBox(height: 16),
                  Text(
                    'QR 扫描需要 camera 权限',
                    style: TextStyle(
                      color: Colors.grey[300],
                      fontSize: 16,
                      fontWeight: FontWeight.w600,
                    ),
                  ),
                  const SizedBox(height: 8),
                  Text(
                    '由于环境限制，相机功能暂不可用。\n请手动输入订阅链接或节点链接。',
                    textAlign: TextAlign.center,
                    style: TextStyle(
                      color: Colors.grey[500],
                      fontSize: 13,
                    ),
                  ),
                ],
              ),
            ),

            const SizedBox(height: 24),

            // Manual input form
            Padding(
              padding: const EdgeInsets.symmetric(horizontal: 24),
              child: Form(
                key: _formKey,
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.stretch,
                  children: [
                    Text(
                      '手动输入订阅链接',
                      style: TextStyle(
                        color: Colors.grey[300],
                        fontSize: 15,
                        fontWeight: FontWeight.w600,
                      ),
                    ),
                    const SizedBox(height: 12),
                    TextFormField(
                      controller: _linkController,
                      maxLines: 3,
                      decoration: InputDecoration(
                        labelText: '订阅链接 / 节点链接',
                        hintText: 'https://example.com/subscribe?token=...',
                        prefixIcon: const Padding(
                          padding: EdgeInsets.only(bottom: 48),
                          child: Icon(Icons.link),
                        ),
                      ),
                      validator: (value) {
                        if (value == null || value.trim().isEmpty) {
                          return '请输入订阅链接';
                        }
                        return null;
                      },
                    ),
                    const SizedBox(height: 8),
                    Text(
                      '支持: HTTP/HTTPS 订阅链接、SS/vMess/vLESS/Trojan/Hysteria2/TUIC 节点链接',
                      style: TextStyle(
                        color: Colors.grey[600],
                        fontSize: 11,
                      ),
                    ),
                    const SizedBox(height: 16),

                    // Error display
                    if (_error != null)
                      Container(
                        padding: const EdgeInsets.all(12),
                        decoration: BoxDecoration(
                          color: Colors.red.withAlpha(25),
                          borderRadius: BorderRadius.circular(8),
                          border: Border.all(color: Colors.red.withAlpha(80)),
                        ),
                        child: Row(
                          children: [
                            const Icon(Icons.error_outline, color: Colors.red, size: 20),
                            const SizedBox(width: 8),
                            Expanded(
                              child: Text(
                                _error!,
                                style: const TextStyle(color: Colors.red, fontSize: 13),
                              ),
                            ),
                          ],
                        ),
                      ),
                    const SizedBox(height: 16),

                    // Import button
                    SizedBox(
                      height: 48,
                      child: ElevatedButton(
                        onPressed: _isImporting ? null : _importLink,
                        child: _isImporting
                            ? const SizedBox(
                                width: 24,
                                height: 24,
                                child: CircularProgressIndicator(
                                  strokeWidth: 2,
                                ),
                              )
                            : const Text('导入', style: TextStyle(fontSize: 16)),
                      ),
                    ),
                  ],
                ),
              ),
            ),
          ],
        ),
      ),
    );
  }
}

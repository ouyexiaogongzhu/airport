import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:provider/provider.dart';
import '../../services/subscription_service.dart';
import 'qr_scanner_page.dart';

class SubscriptionInputPage extends StatefulWidget {
  const SubscriptionInputPage({super.key});

  @override
  State<SubscriptionInputPage> createState() => _SubscriptionInputPageState();
}

class _SubscriptionInputPageState extends State<SubscriptionInputPage> {
  final _urlController = TextEditingController();
  final _formKey = GlobalKey<FormState>();
  bool _isLoading = false;
  bool _showPasteHint = false;

  @override
  void initState() {
    super.initState();
    _detectClipboard();
  }

  @override
  void dispose() {
    _urlController.dispose();
    super.dispose();
  }

  Future<void> _detectClipboard() async {
    try {
      final data = await Clipboard.getData(Clipboard.kTextPlain);
      if (data?.text != null && data!.text!.isNotEmpty) {
        final text = data.text!.trim();
        if (text.startsWith('https://') ||
            text.startsWith('ss://') ||
            text.startsWith('vmess://') ||
            text.startsWith('vless://') ||
            text.startsWith('trojan://') ||
            text.startsWith('hysteria2://') ||
            text.startsWith('tuic://')) {
          _urlController.text = text;
          if (mounted) {
            setState(() => _showPasteHint = true);
          }
        }
      }
    } catch (_) {
      // Clipboard access not available or denied
    }
  }

  Future<void> _importSubscription() async {
    if (!_formKey.currentState!.validate()) return;

    final url = _urlController.text.trim();
    if (url.isEmpty) return;

    setState(() => _isLoading = true);

    try {
      // 无账号模式：直接解析订阅链接 / 节点链接。
      final subService = context.read<SubscriptionService>();
      final success = await subService.importFromLink(url);
      if (!mounted) return;
      if (success) {
        ScaffoldMessenger.of(context).showSnackBar(
          const SnackBar(content: Text('订阅导入成功')),
        );
        if (Navigator.canPop(context)) {
          // 从主界面进入的更新订阅场景：直接返回，节点列表实时刷新。
          Navigator.of(context).pop(true);
        } else {
          // 首次启动进入的导入场景：进入主界面。
          Navigator.of(context).pushReplacementNamed('/main');
        }
      } else {
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(
            content: Text(subService.importError ?? '导入失败'),
            backgroundColor: Colors.red.shade800,
          ),
        );
      }
    } catch (e) {
      if (!mounted) return;
      ScaffoldMessenger.of(context).showSnackBar(
        SnackBar(
          content: Text('导入失败: $e'),
          backgroundColor: Colors.red.shade800,
        ),
      );
    } finally {
      if (mounted) {
        setState(() => _isLoading = false);
      }
    }
  }

  void _openScanner() {
    Navigator.of(context).push(
      MaterialPageRoute(
        builder: (_) => const QrScannerPage(),
      ),
    );
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      appBar: AppBar(
        title: const Text('导入订阅'),
        leading: Navigator.canPop(context)
            ? IconButton(
                icon: const Icon(Icons.arrow_back),
                onPressed: () => Navigator.of(context).pop(),
              )
            : null,
      ),
      body: SingleChildScrollView(
        padding: const EdgeInsets.all(24),
        child: Form(
          key: _formKey,
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.stretch,
            children: [
              // Header icon
              Icon(
                Icons.key,
                size: 64,
                color: Theme.of(context).colorScheme.primary,
              ),
              const SizedBox(height: 16),
              Text(
                '添加订阅链接',
                textAlign: TextAlign.center,
                style: Theme.of(context).textTheme.titleLarge?.copyWith(
                      fontWeight: FontWeight.bold,
                    ),
              ),
              const SizedBox(height: 8),
              Text(
                '输入您的机场订阅链接以导入节点',
                textAlign: TextAlign.center,
                style: TextStyle(
                  color: Colors.grey[400],
                  fontSize: 14,
                ),
              ),
              const SizedBox(height: 32),

              // Clipboard detected hint
              if (_showPasteHint)
                Container(
                  margin: const EdgeInsets.only(bottom: 16),
                  padding: const EdgeInsets.symmetric(
                    horizontal: 12,
                    vertical: 10,
                  ),
                  decoration: BoxDecoration(
                    color: Colors.cyanAccent.withAlpha(20),
                    borderRadius: BorderRadius.circular(8),
                    border: Border.all(
                      color: Colors.cyanAccent.withAlpha(60),
                    ),
                  ),
                  child: Row(
                    children: [
                      Icon(
                        Icons.content_paste_go,
                        size: 18,
                        color: Colors.cyanAccent,
                      ),
                      const SizedBox(width: 8),
                      Expanded(
                        child: Text(
                          '检测到剪贴板中的链接',
                          style: TextStyle(
                            color: Colors.cyanAccent,
                            fontSize: 13,
                          ),
                        ),
                      ),
                    ],
                  ),
                ),

              // URL input field
              TextFormField(
                controller: _urlController,
                maxLines: 3,
                minLines: 1,
                decoration: InputDecoration(
                  labelText: '订阅链接 URL',
                  hintText: 'https://example.com/subscribe?token=...',
                  prefixIcon: const Padding(
                    padding: EdgeInsets.only(bottom: 48),
                    child: Icon(Icons.link),
                  ),
                  suffixIcon: _urlController.text.isNotEmpty
                      ? IconButton(
                          icon: const Icon(Icons.clear, size: 18),
                          onPressed: () {
                            _urlController.clear();
                            setState(() => _showPasteHint = false);
                          },
                        )
                      : null,
                ),
                validator: (value) {
                  if (value == null || value.trim().isEmpty) {
                    return '请输入订阅链接';
                  }
                  final text = value.trim();
                  // SECURITY: reject plain HTTP — subscription token / node
                  // credentials would be sent in cleartext over HTTP.
                  if (text.startsWith('http://')) {
                    return '仅支持 HTTPS 订阅链接';
                  }
                  if (!text.startsWith('https://') &&
                      !text.startsWith('ss://') &&
                      !text.startsWith('vmess://') &&
                      !text.startsWith('vless://') &&
                      !text.startsWith('trojan://') &&
                      !text.startsWith('hysteria2://') &&
                      !text.startsWith('tuic://')) {
                    return '请输入有效的链接';
                  }
                  return null;
                },
                onChanged: (_) => setState(() {}),
              ),
              const SizedBox(height: 8),

              // Supported formats hint
              Text(
                '支持: HTTPS 订阅链接、SS/vMess/vLESS/Trojan/Hysteria2/TUIC 节点链接',
                style: TextStyle(
                  color: Colors.grey[600],
                  fontSize: 11,
                ),
              ),
              const SizedBox(height: 24),

              // Import button
              SizedBox(
                height: 48,
                child: ElevatedButton(
                  onPressed: _isLoading ? null : _importSubscription,
                  child: _isLoading
                      ? const SizedBox(
                          width: 24,
                          height: 24,
                          child: CircularProgressIndicator(
                            strokeWidth: 2,
                            color: Colors.black,
                          ),
                        )
                      : const Text(
                          '导入订阅',
                          style: TextStyle(fontSize: 16),
                        ),
                ),
              ),
              const SizedBox(height: 16),

              // Divider
              Row(
                children: [
                  Expanded(child: Divider(color: Colors.grey[700])),
                  Padding(
                    padding: const EdgeInsets.symmetric(horizontal: 16),
                    child: Text(
                      '或者',
                      style: TextStyle(color: Colors.grey[500], fontSize: 13),
                    ),
                  ),
                  Expanded(child: Divider(color: Colors.grey[700])),
                ],
              ),
              const SizedBox(height: 16),

              // QR scan button
              OutlinedButton.icon(
                onPressed: _openScanner,
                icon: const Icon(Icons.qr_code_scanner),
                label: const Text('扫描二维码'),
                style: OutlinedButton.styleFrom(
                  foregroundColor: Colors.cyanAccent,
                  side: const BorderSide(color: Colors.cyanAccent),
                  padding: const EdgeInsets.symmetric(vertical: 14),
                  shape: RoundedRectangleBorder(
                    borderRadius: BorderRadius.circular(12),
                  ),
                ),
              ),
              const SizedBox(height: 12),

              // Paste from clipboard button
              OutlinedButton.icon(
                onPressed: () async {
                  final data = await Clipboard.getData(Clipboard.kTextPlain);
                  if (data?.text != null && data!.text!.isNotEmpty) {
                    _urlController.text = data.text!.trim();
                    setState(() => _showPasteHint = true);
                  } else {
                    if (mounted) {
                      ScaffoldMessenger.of(context).showSnackBar(
                        const SnackBar(content: Text('剪贴板为空')),
                      );
                    }
                  }
                },
                icon: const Icon(Icons.paste),
                label: const Text('从剪贴板粘贴'),
                style: OutlinedButton.styleFrom(
                  foregroundColor: Colors.grey[300],
                  side: BorderSide(color: Colors.grey[700]!),
                  padding: const EdgeInsets.symmetric(vertical: 14),
                  shape: RoundedRectangleBorder(
                    borderRadius: BorderRadius.circular(12),
                  ),
                ),
              ),
            ],
          ),
        ),
      ),
    );
  }
}

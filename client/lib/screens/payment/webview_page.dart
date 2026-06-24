import 'package:flutter/material.dart';
import 'package:url_launcher/url_launcher.dart';

/// Payment redirect page.
///
/// Opens the [url] in the system browser via [url_launcher].
/// Displays a status card before and after the redirect.
class PaymentWebViewPage extends StatefulWidget {
  /// The portal URL to open (e.g. /plans or /pay/:order_id).
  final String url;

  /// Optional display title.
  final String title;

  const PaymentWebViewPage({
    super.key,
    required this.url,
    this.title = '支付中心',
  });

  @override
  State<PaymentWebViewPage> createState() => _PaymentWebViewPageState();
}

class _PaymentWebViewPageState extends State<PaymentWebViewPage> {
  bool _opened = false;
  bool _failed = false;
  String? _errorMessage;

  @override
  void initState() {
    super.initState();
    // Attempt to open the browser automatically on page load.
    WidgetsBinding.instance.addPostFrameCallback((_) => _openBrowser());
  }

  Future<void> _openBrowser() async {
    final uri = Uri.parse(widget.url);
    try {
      final launched = await launchUrl(uri, mode: LaunchMode.externalApplication);
      if (!mounted) return;
      if (launched) {
        setState(() {
          _opened = true;
          _failed = false;
        });
      } else {
        setState(() {
          _failed = true;
          _errorMessage = '无法打开链接，请手动打开浏览器访问。';
        });
      }
    } catch (e) {
      if (!mounted) return;
      setState(() {
        _failed = true;
        _errorMessage = '打开失败: ${e.toString()}';
      });
    }
  }

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);

    return Scaffold(
      appBar: AppBar(
        title: Text(widget.title),
        leading: IconButton(
          icon: const Icon(Icons.close),
          onPressed: () => Navigator.of(context).pop(),
        ),
      ),
      body: Center(
        child: Padding(
          padding: const EdgeInsets.all(24),
          child: Column(
            mainAxisSize: MainAxisSize.min,
            children: [
              // Status icon
              AnimatedSwitcher(
                duration: const Duration(milliseconds: 300),
                child: _buildStatusIcon(),
              ),
              const SizedBox(height: 24),
              // Status message
              Text(
                _buildStatusMessage(),
                textAlign: TextAlign.center,
                style: TextStyle(
                  fontSize: 16,
                  color: _failed ? Colors.red[300] : Colors.grey[200],
                  height: 1.5,
                ),
              ),
              const SizedBox(height: 8),
              // URL display
              if (widget.url.isNotEmpty)
                Padding(
                  padding: const EdgeInsets.only(top: 12),
                  child: Text(
                    widget.url,
                    textAlign: TextAlign.center,
                    style: TextStyle(
                      fontSize: 12,
                      color: Colors.grey[500],
                    ),
                    maxLines: 2,
                    overflow: TextOverflow.ellipsis,
                  ),
                ),
              const SizedBox(height: 32),
              // Action buttons
              if (!_opened || _failed) ...[
                SizedBox(
                  width: double.infinity,
                  child: ElevatedButton.icon(
                    onPressed: _openBrowser,
                    icon: const Icon(Icons.open_in_browser, size: 20),
                    label: const Text('打开浏览器'),
                    style: ElevatedButton.styleFrom(
                      padding: const EdgeInsets.symmetric(vertical: 14),
                    ),
                  ),
                ),
                const SizedBox(height: 12),
              ],
              if (_opened && !_failed)
                SizedBox(
                  width: double.infinity,
                  child: FilledButton.tonalIcon(
                    onPressed: () => Navigator.of(context).pop(),
                    icon: const Icon(Icons.check, size: 20),
                    label: const Text('已完成支付'),
                    style: FilledButton.styleFrom(
                      padding: const EdgeInsets.symmetric(vertical: 14),
                    ),
                  ),
                ),
              TextButton(
                onPressed: () => Navigator.of(context).pop(),
                child: const Text('返回'),
              ),
            ],
          ),
        ),
      ),
    );
  }

  Widget _buildStatusIcon() {
    if (_failed) {
      return Icon(Icons.error_outline, size: 72, color: Colors.red[300]);
    }
    if (_opened) {
      return const Icon(Icons.check_circle_outline, size: 72, color: Colors.green);
    }
    return const SizedBox(
      width: 72,
      height: 72,
      child: CircularProgressIndicator(strokeWidth: 3),
    );
  }

  String _buildStatusMessage() {
    if (_failed) {
      return _errorMessage ?? '无法打开支付页面';
    }
    if (_opened) {
      return '已打开浏览器\n请在浏览器中完成支付，完成后返回此页面。';
    }
    return '正在打开支付页面…';
  }
}

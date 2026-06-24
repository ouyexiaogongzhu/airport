import 'package:flutter/material.dart';
import 'package:mobile_scanner/mobile_scanner.dart';
import 'package:provider/provider.dart';
import '../../services/subscription_service.dart';

/// QR scanner page: scans a QR code and auto-imports subscription.
///
/// Returns `true` via Navigator.pop if import succeeded.
class QrScannerPage extends StatefulWidget {
  const QrScannerPage({super.key});

  @override
  State<QrScannerPage> createState() => _QrScannerPageState();
}

class _QrScannerPageState extends State<QrScannerPage> {
  bool _scanned = false;
  String? _error;
  bool _cameraAvailable = true;

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
      body: Column(
        children: [
          // Camera preview area
          Expanded(
            child: _cameraAvailable
                ? _buildCameraPreview()
                : _buildCameraUnavailable(),
          ),

          // Error display
          if (_error != null)
            Padding(
              padding: const EdgeInsets.all(16),
              child: Container(
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
            ),

          // Bottom info / manual input
          Padding(
            padding: const EdgeInsets.all(24),
            child: Column(
              children: [
                Icon(Icons.qr_code, size: 32, color: Colors.grey[500]),
                const SizedBox(height: 8),
                Text(
                  '将二维码对准相机框内自动扫描',
                  style: TextStyle(color: Colors.grey[400], fontSize: 14),
                ),
                const SizedBox(height: 16),
                Text(
                  '也可以手动输入订阅链接',
                  style: TextStyle(color: Colors.grey[600], fontSize: 12),
                ),
              ],
            ),
          ),
        ],
      ),
    );
  }

  Widget _buildCameraPreview() {
    try {
      return _MobileScannerWidget(
        onScan: _handleScan,
        onCameraError: _handleCameraError,
      );
    } catch (e) {
      return _buildCameraUnavailable();
    }
  }

  Widget _buildCameraUnavailable() {
    return Center(
      child: Column(
        mainAxisSize: MainAxisSize.min,
        children: [
          Icon(Icons.videocam_off, size: 64, color: Colors.grey[600]),
          const SizedBox(height: 16),
          Text(
            '相机不可用',
            style: TextStyle(color: Colors.grey[400], fontSize: 16),
          ),
          const SizedBox(height: 8),
          Text(
            '请检查相机权限或使用手动输入',
            style: TextStyle(color: Colors.grey[600], fontSize: 13),
          ),
        ],
      ),
    );
  }

  void _handleCameraError() {
    if (mounted) {
      setState(() => _cameraAvailable = false);
    }
  }

  void _handleScan(String text) async {
    if (_scanned) return; // prevent double-trigger
    _scanned = true;
    setState(() => _error = null);

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
        _error = subService.importError ?? '无法解析二维码内容';
        _scanned = false; // allow retry
      });
    }
  }
}

// ---------------------------------------------------------------------------
// Mobile Scanner widget — wraps mobile_scanner package
// ---------------------------------------------------------------------------
class _MobileScannerWidget extends StatelessWidget {
  final void Function(String text) onScan;
  final VoidCallback? onCameraError;

  const _MobileScannerWidget({
    required this.onScan,
    this.onCameraError,
  });

  @override
  Widget build(BuildContext context) {
    // Use MobileScanner from the package
    return MobileScanner(
      fit: BoxFit.cover,
      errorBuilder: (context, error, child) {
        WidgetsBinding.instance.addPostFrameCallback((_) {
          onCameraError?.call();
        });
        return Center(
          child: Column(
            mainAxisSize: MainAxisSize.min,
            children: [
              Icon(Icons.videocam_off, size: 64, color: Colors.grey[600]),
              const SizedBox(height: 16),
              Text(
                '相机错误: ${error.errorCode}',
                style: TextStyle(color: Colors.grey[400]),
              ),
            ],
          ),
        );
      },
      onDetect: (capture) {
        final barcode = capture.barcodes.firstOrNull;
        if (barcode != null && barcode.rawValue != null) {
          onScan(barcode.rawValue!);
        }
      },
    );
  }
}

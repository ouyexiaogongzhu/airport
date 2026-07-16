import 'package:flutter/material.dart';
import '../../widgets/status_badge.dart';

class _DeviceInfo {
  final String name;
  final String ip;
  final String lastActive;
  final String status;
  final bool online;

  const _DeviceInfo({
    required this.name,
    required this.ip,
    required this.lastActive,
    required this.status,
    required this.online,
  });
}

const _placeholderDevices = [
  _DeviceInfo(
    name: 'MacBook Pro',
    ip: '192.168.1.12',
    lastActive: '刚刚',
    status: '在线',
    online: true,
  ),
  _DeviceInfo(
    name: 'iPhone 15',
    ip: '10.0.0.45',
    lastActive: '5 分钟前',
    status: '在线',
    online: true,
  ),
  _DeviceInfo(
    name: 'Windows PC',
    ip: '172.16.0.88',
    lastActive: '2 小时前',
    status: '离线',
    online: false,
  ),
  _DeviceInfo(
    name: 'iPad Air',
    ip: '192.168.1.30',
    lastActive: '昨天',
    status: '离线',
    online: false,
  ),
];

class DeviceList extends StatelessWidget {
  const DeviceList({super.key});

  StatusBadge _deviceStatusBadge(_DeviceInfo device) {
    if (device.online) {
      return StatusBadge.active(label: device.status);
    }
    return StatusBadge.offline(label: device.status);
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      appBar: AppBar(
        title: const Text('设备管理'),
        leading: IconButton(
          icon: const Icon(Icons.arrow_back),
          onPressed: () => Navigator.pop(context),
        ),
      ),
      body: ListView(
        padding: const EdgeInsets.all(16),
        children: [
        // TODO(L3): Device list uses hardcoded placeholder data. Replace with
        // real device data from the API when the devices feature is implemented.
        Container(
          padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 8),
          decoration: BoxDecoration(
            color: Colors.amber.withAlpha(30),
            borderRadius: BorderRadius.circular(8),
            border: Border.all(color: Colors.amber.withAlpha(80)),
          ),
          child: Row(
            children: [
              const Icon(Icons.construction, color: Colors.amber, size: 18),
              const SizedBox(width: 8),
              Expanded(
                child: Text(
                  '功能开发中 — 以下为示例数据',
                  style: TextStyle(color: Colors.amber[300], fontSize: 13),
                ),
              ),
            ],
          ),
        ),
        const SizedBox(height: 16),
        Text(
          '设备管理',
          style: Theme.of(context).textTheme.headlineSmall?.copyWith(
                fontWeight: FontWeight.bold,
              ),
        ),
        const SizedBox(height: 8),
        Text(
          '查看已连接的设备',
          style: TextStyle(color: Colors.grey[400]),
        ),
        const SizedBox(height: 24),
        Card(
          child: Padding(
            padding: const EdgeInsets.all(16),
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Row(
                  children: [
                    Icon(
                      Icons.devices,
                      color: Theme.of(context).colorScheme.primary,
                    ),
                    const SizedBox(width: 8),
                    const Text(
                      'Connected Devices',
                      style: TextStyle(
                        fontSize: 18,
                        fontWeight: FontWeight.bold,
                      ),
                    ),
                  ],
                ),
                const SizedBox(height: 16),
                ..._placeholderDevices.map(
                  (device) => Padding(
                    padding: const EdgeInsets.only(bottom: 12),
                    child: Container(
                      padding: const EdgeInsets.all(12),
                      decoration: BoxDecoration(
                        color: Theme.of(context).colorScheme.surfaceContainerHighest,
                        borderRadius: BorderRadius.circular(12),
                      ),
                      child: Row(
                        children: [
                          CircleAvatar(
                            radius: 20,
                            backgroundColor:
                                Theme.of(context).colorScheme.primary.withAlpha(30),
                            child: Icon(
                              Icons.smartphone,
                              size: 20,
                              color: Theme.of(context).colorScheme.primary,
                            ),
                          ),
                          const SizedBox(width: 12),
                          Expanded(
                            child: Column(
                              crossAxisAlignment: CrossAxisAlignment.start,
                              children: [
                                Text(
                                  device.name,
                                  style: const TextStyle(
                                    fontWeight: FontWeight.w600,
                                    fontSize: 15,
                                  ),
                                ),
                                const SizedBox(height: 2),
                                Text(
                                  device.ip,
                                  style: TextStyle(
                                    color: Colors.grey[400],
                                    fontSize: 12,
                                  ),
                                ),
                                const SizedBox(height: 2),
                                Text(
                                  '最后活跃: ${device.lastActive}',
                                  style: TextStyle(
                                    color: Colors.grey[500],
                                    fontSize: 11,
                                  ),
                                ),
                              ],
                            ),
                          ),
                          _deviceStatusBadge(device),
                        ],
                      ),
                    ),
                  ),
                ),
              ],
            ),
          ),
        ),
        ],
      ),
    );
  }
}

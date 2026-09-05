class VpnNode {
  final String name;
  final String uri;

  VpnNode({required this.name, required this.uri});

  factory VpnNode.fromUri(String uri, int index) {
    return VpnNode(
      name: 'Node-${index + 1}',
      uri: uri,
    );
  }
}

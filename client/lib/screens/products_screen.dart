import 'package:flutter/material.dart';
import '../models/product.dart';
import '../services/api_service.dart';

class ProductsScreen extends StatefulWidget {
  const ProductsScreen({super.key});

  @override
  State<ProductsScreen> createState() => _ProductsScreenState();
}

class _ProductsScreenState extends State<ProductsScreen> {
  List<Product> _products = [];
  bool _loading = true;
  final _mockProducts = [
    Product(
      id: 1,
      name: 'Premium VPN 月卡',
      description: '高速稳定 VPN 服务，支持全球节点，不限流量',
      price: 29.99,
      category: 'VPN',
      imageUrl: null,
      status: 'active',
      stock: 999,
      createdAt: '2025-01-15T00:00:00Z',
    ),
    Product(
      id: 2,
      name: 'Premium VPN 季卡',
      description: '三个月超值套餐，节省 20%',
      price: 79.99,
      category: 'VPN',
      imageUrl: null,
      status: 'active',
      stock: 500,
      createdAt: '2025-01-15T00:00:00Z',
    ),
    Product(
      id: 3,
      name: '流媒体加速包',
      description: '专为 Netflix / Disney+ / YouTube 优化的加速服务',
      price: 19.99,
      category: '加速器',
      imageUrl: null,
      status: 'active',
      stock: 200,
      createdAt: '2025-02-01T00:00:00Z',
    ),
    Product(
      id: 4,
      name: '游戏加速专业版',
      description: '低延迟游戏加速，支持 200+ 游戏',
      price: 24.99,
      category: '加速器',
      imageUrl: null,
      status: 'active',
      stock: 350,
      createdAt: '2025-02-10T00:00:00Z',
    ),
    Product(
      id: 5,
      name: '团队版 - 5人',
      description: '5 人共享团队账号，独立线路',
      price: 99.99,
      category: '团队',
      imageUrl: null,
      status: 'inactive',
      stock: 50,
      createdAt: '2025-03-01T00:00:00Z',
    ),
    Product(
      id: 6,
      name: '企业定制方案',
      description: '企业级定制 VPN 方案，专用服务器，SLA 保障',
      price: 499.99,
      category: '企业',
      imageUrl: null,
      status: 'active',
      stock: 10,
      createdAt: '2025-03-15T00:00:00Z',
    ),
  ];

  @override
  void initState() {
    super.initState();
    _loadProducts();
  }

  Future<void> _loadProducts() async {
    setState(() => _loading = true);
    try {
      final data = await ApiService().get('/products', queryParams: {'page': '1', 'size': '20'});
      final list = (data['data'] as List<dynamic>? ?? [])
          .map((e) => Product.fromJson(e as Map<String, dynamic>))
          .toList();
      setState(() {
        _products = list;
        _loading = false;
      });
    } catch (_) {
      setState(() {
        _products = _mockProducts;
        _loading = false;
      });
    }
  }

  String _price(double price) => price.toStringAsFixed(2);

  @override
  Widget build(BuildContext context) {
    return RefreshIndicator(
      onRefresh: _loadProducts,
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Padding(
            padding: const EdgeInsets.fromLTRB(16, 16, 16, 8),
            child: Row(
              mainAxisAlignment: MainAxisAlignment.spaceBetween,
              children: [
                Text(
                  '产品列表',
                  style: Theme.of(context).textTheme.headlineSmall?.copyWith(
                        fontWeight: FontWeight.bold,
                      ),
                ),
                Text(
                  '共 ${_products.length} 个产品',
                  style: TextStyle(color: Colors.grey[400], fontSize: 13),
                ),
              ],
            ),
          ),
          const Padding(
            padding: EdgeInsets.symmetric(horizontal: 16),
            child: Text(
              '所有可用产品与套餐',
              style: TextStyle(color: Colors.grey, fontSize: 13),
            ),
          ),
          const SizedBox(height: 8),
          Expanded(
            child: _loading
                ? const Center(child: CircularProgressIndicator())
                : ListView.builder(
                    padding: const EdgeInsets.symmetric(horizontal: 16),
                    itemCount: _products.length,
                    itemBuilder: (context, index) {
                      final product = _products[index];
                      return _ProductCard(product: product, price: _price);
                    },
                  ),
          ),
        ],
      ),
    );
  }
}

class _ProductCard extends StatelessWidget {
  final Product product;
  final String Function(double) price;

  const _ProductCard({required this.product, required this.price});

  @override
  Widget build(BuildContext context) {
    final isActive = product.status == 'active';
    return Card(
      margin: const EdgeInsets.only(bottom: 12),
      shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(12)),
      child: Padding(
        padding: const EdgeInsets.all(16),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Row(
              children: [
                Container(
                  width: 48,
                  height: 48,
                  decoration: BoxDecoration(
                    color: isActive
                        ? Theme.of(context).colorScheme.primary.withAlpha(30)
                        : Colors.grey.withAlpha(30),
                    borderRadius: BorderRadius.circular(12),
                  ),
                  child: Icon(
                    _categoryIcon(product.category),
                    color: isActive
                        ? Theme.of(context).colorScheme.primary
                        : Colors.grey,
                  ),
                ),
                const SizedBox(width: 12),
                Expanded(
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      Text(
                        product.name,
                        style: const TextStyle(
                          fontWeight: FontWeight.w600,
                          fontSize: 15,
                        ),
                      ),
                      const SizedBox(height: 4),
                      Text(
                        product.category,
                        style: TextStyle(
                          color: Colors.grey[500],
                          fontSize: 12,
                        ),
                      ),
                    ],
                  ),
                ),
                Container(
                  padding: const EdgeInsets.symmetric(horizontal: 8, vertical: 4),
                  decoration: BoxDecoration(
                    color: isActive ? Colors.green.withAlpha(25) : Colors.red.withAlpha(25),
                    borderRadius: BorderRadius.circular(6),
                  ),
                  child: Text(
                    isActive ? '上架中' : '已下架',
                    style: TextStyle(
                      color: isActive ? Colors.green : Colors.red,
                      fontSize: 11,
                      fontWeight: FontWeight.w600,
                    ),
                  ),
                ),
              ],
            ),
            const SizedBox(height: 12),
            Text(
              product.description,
              style: TextStyle(color: Colors.grey[400], fontSize: 13),
            ),
            const SizedBox(height: 12),
            Row(
              mainAxisAlignment: MainAxisAlignment.spaceBetween,
              children: [
                Text(
                  '¥${price(product.price)}',
                  style: TextStyle(
                    fontSize: 20,
                    fontWeight: FontWeight.bold,
                    color: Theme.of(context).colorScheme.primary,
                  ),
                ),
                Text(
                  '库存: ${product.stock}',
                  style: TextStyle(color: Colors.grey[500], fontSize: 12),
                ),
              ],
            ),
          ],
        ),
      ),
    );
  }

  IconData _categoryIcon(String category) {
    switch (category) {
      case 'VPN':
        return Icons.vpn_lock;
      case '加速器':
        return Icons.speed;
      case '团队':
        return Icons.group;
      case '企业':
        return Icons.business;
      default:
        return Icons.inventory_2;
    }
  }
}

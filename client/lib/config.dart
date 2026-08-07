/// 编译期构建配置（通过 --dart-define 注入）。
///
/// STORE_MODE=true 时构建 App Store 版「通用代理客户端」：
/// App 内零账号（无登录/注册/token-login）、零金流（无购买/续费/官网跳转），
/// 只能通过「粘贴订阅 URL / 扫描二维码」导入节点。
class AppConfig {
  static const bool storeMode = bool.fromEnvironment('STORE_MODE');
}

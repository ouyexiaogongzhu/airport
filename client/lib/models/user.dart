// TODO(L1): Implement operator == and hashCode for value-based equality.
// Consider using package:equatable or manually overriding these.
class User {
  final int id;
  final String username;
  final String email;
  final String role;
  final String? token;
  final String createdAt;

  User({
    required this.id,
    required this.username,
    required this.email,
    required this.role,
    this.token,
    required this.createdAt,
  });

  factory User.fromJson(Map<String, dynamic> json) {
    return User(
      id: json['id'] as int,
      username: json['username'] as String? ?? '',
      email: json['email'] as String? ?? '',
      role: json['role'] as String? ?? 'user',
      token: json['token'] as String?,
      createdAt: json['created_at'] as String? ?? '',
    );
  }

  Map<String, dynamic> toJson() {
    return {
      'id': id,
      'username': username,
      'email': email,
      'role': role,
      'token': token,
      'created_at': createdAt,
    };
  }
}

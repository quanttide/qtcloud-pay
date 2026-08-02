import 'package:flutter/material.dart';
import 'package:provider/provider.dart';

import '../services/pay_api.dart';
import '../services/pay_store.dart';
import '../widgets/money_text.dart';

/// 账户页：本会话账户列表、创建账户。
class AccountsScreen extends StatefulWidget {
  final void Function(String accountId) onOpenDetail;

  const AccountsScreen({super.key, required this.onOpenDetail});

  @override
  State<AccountsScreen> createState() => _AccountsScreenState();
}

class _AccountsScreenState extends State<AccountsScreen> {
  final _formKey = GlobalKey<FormState>();
  final _customerIdCtrl = TextEditingController();
  bool _creating = false;

  @override
  void dispose() {
    _customerIdCtrl.dispose();
    super.dispose();
  }

  Future<void> _create() async {
    if (!_formKey.currentState!.validate()) return;
    setState(() => _creating = true);
    try {
      final store = context.read<PayStore>();
      final account = await store.api.createAccount(_customerIdCtrl.text.trim());
      store.cacheAccount(account);
      if (!mounted) return;
      ScaffoldMessenger.of(context).showSnackBar(
        SnackBar(content: Text('已创建账户 ${account.id}')),
      );
      _customerIdCtrl.clear();
    } on ApiException catch (e) {
      if (!mounted) return;
      ScaffoldMessenger.of(context)
          .showSnackBar(SnackBar(content: Text('创建失败：${e.message}')));
    } finally {
      if (mounted) setState(() => _creating = false);
    }
  }

  @override
  Widget build(BuildContext context) {
    final store = context.watch<PayStore>();
    final accounts = store.accounts;
    return Padding(
      padding: const EdgeInsets.all(24),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Text('账户', style: Theme.of(context).textTheme.headlineMedium),
          const SizedBox(height: 16),
          Card(
            child: Padding(
              padding: const EdgeInsets.all(16),
              child: Form(
                key: _formKey,
                child: Row(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    Expanded(
                      child: TextFormField(
                        controller: _customerIdCtrl,
                        decoration: const InputDecoration(
                          labelText: '客户 ID',
                          hintText: '如 cus_xxx',
                          border: OutlineInputBorder(),
                        ),
                        validator: (v) => (v == null || v.trim().isEmpty)
                            ? '请输入客户 ID'
                            : null,
                      ),
                    ),
                    const SizedBox(width: 12),
                    FilledButton.icon(
                      onPressed: _creating ? null : _create,
                      icon: _creating
                          ? const SizedBox(
                              width: 16,
                              height: 16,
                              child: CircularProgressIndicator(strokeWidth: 2),
                            )
                          : const Icon(Icons.add),
                      label: const Text('创建账户'),
                    ),
                  ],
                ),
              ),
            ),
          ),
          const SizedBox(height: 16),
          Text('最近账户（本会话）',
              style: Theme.of(context).textTheme.titleMedium),
          const SizedBox(height: 8),
          if (accounts.isEmpty)
            const Expanded(child: Center(child: Text('暂无账户，请先创建')))
          else
            Expanded(
              child: ListView.builder(
                itemCount: accounts.length,
                itemBuilder: (context, i) {
                  final a = accounts[i];
                  return Card(
                    margin: const EdgeInsets.only(bottom: 8),
                    child: ListTile(
                      leading: const Icon(Icons.account_balance_wallet),
                      title: Text('${a.id} · 客户 ${a.customerId}'),
                      subtitle: Text('创建于 ${a.createdAt}'),
                      trailing: Row(
                        mainAxisSize: MainAxisSize.min,
                        children: [
                          Text('余额 '),
                          MoneyText(a.balance),
                          IconButton(
                            tooltip: '查看详情',
                            icon: const Icon(Icons.chevron_right),
                            onPressed: () => widget.onOpenDetail(a.id),
                          ),
                        ],
                      ),
                    ),
                  );
                },
              ),
            ),
        ],
      ),
    );
  }
}

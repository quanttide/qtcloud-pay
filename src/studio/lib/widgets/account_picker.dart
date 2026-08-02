import 'package:flutter/material.dart';
import 'package:provider/provider.dart';

import '../services/pay_store.dart';

/// 账户选择：文本输入账户 id + 下拉快捷选择（本会话已缓存账户）。
///
/// 服务端 v0.1.0 无 GET /accounts 列表端点，因此以手动输入为主，
/// 缓存列表用于快捷选取，避免手工抄录账户号。可在外层 [Form] 中
/// 通过 [onSaved] / [validator] 参与表单校验与保存。
class AccountPicker extends StatefulWidget {
  final FormFieldSetter<String>? onSaved;
  final FormFieldValidator<String>? validator;
  final String labelText;

  const AccountPicker({
    super.key,
    this.onSaved,
    this.validator,
    this.labelText = '账户 ID',
  });

  @override
  State<AccountPicker> createState() => _AccountPickerState();
}

class _AccountPickerState extends State<AccountPicker> {
  late final TextEditingController _controller = TextEditingController();

  @override
  void dispose() {
    _controller.dispose();
    super.dispose();
  }

  void _pick(String accountId) {
    setState(() => _controller.text = accountId);
  }

  @override
  Widget build(BuildContext context) {
    final store = context.watch<PayStore>();
    final accounts = store.accounts;
    return TextFormField(
      controller: _controller,
      decoration: InputDecoration(
        labelText: widget.labelText,
        hintText: '如 acc_xxx',
        border: const OutlineInputBorder(),
        suffixIcon: accounts.isEmpty
            ? null
            : PopupMenuButton<String>(
                tooltip: '选择最近账户',
                onSelected: _pick,
                itemBuilder: (_) => [
                  for (final acc in accounts)
                    PopupMenuItem(
                      value: acc.id,
                      child: Text('${acc.id}（${acc.customerId}）'),
                    ),
                ],
              ),
      ),
      onSaved: widget.onSaved,
      validator: (v) {
        if (v == null || v.trim().isEmpty) return '请选择或输入账户 ID';
        return widget.validator?.call(v);
      },
    );
  }
}

import 'package:flutter/material.dart';
import 'package:provider/provider.dart';

import '../services/pay_api.dart';
import '../services/pay_store.dart';
import '../widgets/account_picker.dart';
import '../widgets/amount_field.dart';
import '../widgets/idempotency_field.dart';
import '../widgets/money_text.dart';

/// 充值登记页：对公打款入账（账户 + 金额 + 打款凭证号幂等键）。
class RechargeScreen extends StatefulWidget {
  const RechargeScreen({super.key});

  @override
  State<RechargeScreen> createState() => _RechargeScreenState();
}

class _RechargeScreenState extends State<RechargeScreen> {
  final _formKey = GlobalKey<FormState>();
  String? _accountId;
  final _amountCtrl = TextEditingController();
  final _voucherNoCtrl = TextEditingController();
  final _noteCtrl = TextEditingController();
  bool _submitting = false;

  @override
  void dispose() {
    _amountCtrl.dispose();
    _voucherNoCtrl.dispose();
    _noteCtrl.dispose();
    super.dispose();
  }

  Future<void> _submit() async {
    if (!_formKey.currentState!.validate()) return;
    setState(() => _submitting = true);
    try {
      final store = context.read<PayStore>();
      final cents = AmountField.yuanToCents(_amountCtrl.text);
      await store.api.recharge(
        _accountId!,
        amount: cents,
        voucherNo: _voucherNoCtrl.text.trim(),
        note: _noteCtrl.text.trim().isEmpty ? null : _noteCtrl.text.trim(),
      );
      // 刷新账户余额
      final account = await store.api.getAccount(_accountId!);
      final moneyText = MoneyText.format(cents);
      store.cacheAccount(account);
      if (!mounted) return;
      showDialog<void>(
        context: context,
        builder: (_) => AlertDialog(
          title: const Text('充值登记成功'),
          content: Text(
            '账户 $_accountId 已入账，当前余额 '
            '¥$moneyText。重复提交同凭证号不会重复入账（幂等）。',
          ),
          actions: [
            TextButton(
              onPressed: () => Navigator.of(context).pop(),
              child: const Text('完成'),
            ),
          ],
        ),
      );
      _amountCtrl.clear();
      _voucherNoCtrl.clear();
      _noteCtrl.clear();
    } on ApiException catch (e) {
      if (!mounted) return;
      ScaffoldMessenger.of(context)
          .showSnackBar(SnackBar(content: Text('登记失败：${e.message}')));
    } finally {
      if (mounted) setState(() => _submitting = false);
    }
  }

  @override
  Widget build(BuildContext context) {
    return SingleChildScrollView(
      padding: const EdgeInsets.all(24),
      child: ConstrainedBox(
        constraints: const BoxConstraints(maxWidth: 560),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Text('充值登记（对公打款入账）',
                style: Theme.of(context).textTheme.headlineMedium),
            const SizedBox(height: 4),
            Text('打款凭证号即幂等键：同一凭证号重复登记不会生效两次',
                style: TextStyle(color: Colors.grey.shade600)),
            const SizedBox(height: 16),
            Form(
              key: _formKey,
              child: Column(
                children: [
                  AccountPicker(
                    onSaved: (v) => _accountId = v,
                    validator: (v) => (v == null || v.trim().isEmpty)
                        ? '请选择或输入账户 ID'
                        : null,
                  ),
                  const SizedBox(height: 16),
                  AmountField(controller: _amountCtrl),
                  const SizedBox(height: 16),
                  IdempotencyField(
                    controller: _voucherNoCtrl,
                    labelText: '打款凭证号',
                    hintText: '如 V20260803001',
                  ),
                  const SizedBox(height: 16),
                  TextFormField(
                    controller: _noteCtrl,
                    decoration: const InputDecoration(
                      labelText: '备注（可选）',
                      border: OutlineInputBorder(),
                    ),
                  ),
                  const SizedBox(height: 24),
                  SizedBox(
                    width: double.infinity,
                    child: FilledButton.icon(
                      onPressed: _submitting ? null : _submit,
                      icon: _submitting
                          ? const SizedBox(
                              width: 16,
                              height: 16,
                              child: CircularProgressIndicator(strokeWidth: 2),
                            )
                          : const Icon(Icons.arrow_downward),
                      label: const Text('登记入账'),
                    ),
                  ),
                ],
              ),
            ),
          ],
        ),
      ),
    );
  }
}


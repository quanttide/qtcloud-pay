import 'package:flutter/material.dart';
import 'package:provider/provider.dart';

import '../models/order.dart';
import '../services/pay_api.dart';
import '../services/pay_store.dart';
import '../widgets/account_picker.dart';
import '../widgets/amount_field.dart';
import '../widgets/idempotency_field.dart';
import '../widgets/settle_detail_panel.dart';
import '../widgets/status_chip.dart';

/// 订单结算页：下单并结算（幂等键 = 商户订单号），展示结算明细。
class OrderScreen extends StatefulWidget {
  const OrderScreen({super.key});

  @override
  State<OrderScreen> createState() => _OrderScreenState();
}

class _OrderScreenState extends State<OrderScreen> {
  final _formKey = GlobalKey<FormState>();
  final _orderIdCtrl = TextEditingController();
  final _customerIdCtrl = TextEditingController();
  String? _accountId;
  String? _scope;
  final _productIdCtrl = TextEditingController();
  final _amountCtrl = TextEditingController();
  bool _submitting = false;
  Order? _lastOrder;

  @override
  void dispose() {
    _orderIdCtrl.dispose();
    _customerIdCtrl.dispose();
    _productIdCtrl.dispose();
    _amountCtrl.dispose();
    super.dispose();
  }

  Future<void> _submit() async {
    if (!_formKey.currentState!.validate()) return;
    setState(() => _submitting = true);
    try {
      final store = context.read<PayStore>();
      final order = await store.api.settleOrder(
        orderId: _orderIdCtrl.text.trim(),
        customerId: _customerIdCtrl.text.trim(),
        accountId: _accountId!,
        scope: _scope,
        productId: _productIdCtrl.text.trim().isEmpty
            ? null
            : _productIdCtrl.text.trim(),
        amount: AmountField.yuanToCents(_amountCtrl.text),
      );
      store.cacheOrder(order);
      // 刷新账户余额
      final account = await store.api.getAccount(_accountId!);
      store.cacheAccount(account);
      if (!mounted) return;
      setState(() => _lastOrder = order);
      _orderIdCtrl.clear();
      _amountCtrl.clear();
      _customerIdCtrl.clear();
    } on ApiException catch (e) {
      if (!mounted) return;
      ScaffoldMessenger.of(context).showSnackBar(
        SnackBar(
          content: Text(e.statusCode == 422
              ? '结算失败：余额不足，请先充值（订单未生成）'
              : '结算失败：${e.message}'),
        ),
      );
    } finally {
      if (mounted) setState(() => _submitting = false);
    }
  }

  @override
  Widget build(BuildContext context) {
    return SingleChildScrollView(
      padding: const EdgeInsets.all(24),
      child: ConstrainedBox(
        constraints: const BoxConstraints(maxWidth: 640),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Text('订单结算', style: Theme.of(context).textTheme.headlineMedium),
            const SizedBox(height: 4),
            Text('商户订单号即幂等键：同一订单号重复提交不会重复结算；余额不足将拒绝（422）',
                style: TextStyle(color: Colors.grey.shade600)),
            const SizedBox(height: 16),
            Card(
              child: Padding(
                padding: const EdgeInsets.all(16),
                child: Form(
                  key: _formKey,
                  child: Column(
                    children: [
                      IdempotencyField(
                        controller: _orderIdCtrl,
                        labelText: '商户订单号',
                        hintText: '如 ORD20260803001',
                      ),
                      const SizedBox(height: 16),
                      TextFormField(
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
                      const SizedBox(height: 16),
                      AccountPicker(
                        onSaved: (v) => _accountId = v,
                        validator: (v) => (v == null || v.trim().isEmpty)
                            ? '请选择或输入账户 ID'
                            : null,
                      ),
                      const SizedBox(height: 16),
                      DropdownButtonFormField<String>(
                        initialValue: _scope,
                        decoration: const InputDecoration(
                          labelText: '业务类型（可选）',
                          border: OutlineInputBorder(),
                        ),
                        items: const [
                          DropdownMenuItem(value: 'cloud', child: Text('云服务')),
                          DropdownMenuItem(value: 'course', child: Text('课程')),
                          DropdownMenuItem(value: 'data', child: Text('数据服务')),
                        ],
                        onChanged: (v) => setState(() => _scope = v),
                      ),
                      const SizedBox(height: 16),
                      TextFormField(
                        controller: _productIdCtrl,
                        decoration: const InputDecoration(
                          labelText: '商品 ID（可选）',
                          hintText: '如 prod_xxx',
                          border: OutlineInputBorder(),
                        ),
                      ),
                      const SizedBox(height: 16),
                      AmountField(controller: _amountCtrl, labelText: '订单金额（元）'),
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
                              : const Icon(Icons.payment),
                          label: const Text('下单并结算'),
                        ),
                      ),
                    ],
                  ),
                ),
              ),
            ),
            if (_lastOrder != null) ...[
              const SizedBox(height: 16),
              Card(
                child: Padding(
                  padding: const EdgeInsets.all(16),
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      Row(
                        children: [
                          Expanded(
                            child: Text('订单 ${_lastOrder!.id}',
                                style:
                                    Theme.of(context).textTheme.titleMedium),
                          ),
                          StatusChip(status: _lastOrder!.status),
                        ],
                      ),
                      const SizedBox(height: 4),
                      Text('下单时间 ${_lastOrder!.createdAt}',
                          style: TextStyle(color: Colors.grey.shade600)),
                      const SizedBox(height: 12),
                      SettleDetailPanel(
                        deductions: _lastOrder!.settleDetail,
                        orderAmount: _lastOrder!.amount,
                      ),
                    ],
                  ),
                ),
              ),
            ],
          ],
        ),
      ),
    );
  }
}

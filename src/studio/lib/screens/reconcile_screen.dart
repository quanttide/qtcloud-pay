import 'package:flutter/material.dart';
import 'package:provider/provider.dart';

import '../models/reconciliation.dart';
import '../services/pay_api.dart';
import '../services/pay_store.dart';
import '../widgets/account_picker.dart';
import '../widgets/money_text.dart';
import '../widgets/reconcile_diff_table.dart';

/// 对账页：一致性校验、对公打款核对（CSV）、账单导出。
class ReconcileScreen extends StatefulWidget {
  final void Function(String accountId) onOpenAccount;

  const ReconcileScreen({super.key, required this.onOpenAccount});

  @override
  State<ReconcileScreen> createState() => _ReconcileScreenState();
}

class _ReconcileScreenState extends State<ReconcileScreen> {
  List<Discrepancy>? _discrepancies;
  bool _checking = false;
  String? _checkError;

  final _bankCsvCtrl = TextEditingController();
  BankReport? _bankReport;
  bool _uploading = false;
  String? _uploadError;

  String? _statementAccountId;
  Statement? _statement;
  bool _loadingStatement = false;
  String? _statementError;

  @override
  void dispose() {
    _bankCsvCtrl.dispose();
    super.dispose();
  }

  Future<void> _checkConsistency() async {
    setState(() {
      _checking = true;
      _checkError = null;
    });
    try {
      final store = context.read<PayStore>();
      final result = await store.api.checkConsistency();
      if (!mounted) return;
      setState(() => _discrepancies = result);
    } on ApiException catch (e) {
      if (!mounted) return;
      setState(() => _checkError = '校验失败：${e.message}');
    } finally {
      if (mounted) setState(() => _checking = false);
    }
  }

  Future<void> _uploadBank() async {
    final csv = _bankCsvCtrl.text.trim();
    if (csv.isEmpty) {
      setState(() => _uploadError = '请粘贴银行流水 CSV 内容');
      return;
    }
    setState(() {
      _uploading = true;
      _uploadError = null;
    });
    try {
      final store = context.read<PayStore>();
      final report = await store.api.reconcileBankFile(csv);
      if (!mounted) return;
      setState(() => _bankReport = report);
    } on ApiException catch (e) {
      if (!mounted) return;
      setState(() => _uploadError = '核对失败：${e.message}');
    } finally {
      if (mounted) setState(() => _uploading = false);
    }
  }

  Future<void> _loadStatement() async {
    final accountId = _statementAccountId;
    if (accountId == null) {
      setState(() => _statementError = '请输入账户 ID');
      return;
    }
    setState(() {
      _loadingStatement = true;
      _statementError = null;
    });
    try {
      final store = context.read<PayStore>();
      final stmt = await store.api.getStatement(accountId);
      if (!mounted) return;
      setState(() => _statement = stmt);
    } on ApiException catch (e) {
      if (!mounted) return;
      setState(() => _statementError = '账单加载失败：${e.message}');
    } finally {
      if (mounted) setState(() => _loadingStatement = false);
    }
  }

  @override
  Widget build(BuildContext context) {
    return SingleChildScrollView(
      padding: const EdgeInsets.all(24),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Text('对账与可查', style: Theme.of(context).textTheme.headlineMedium),
          const SizedBox(height: 4),
          Text('一致性校验（不错）· 对公打款核对（可查）· 账单导出',
              style: TextStyle(color: Colors.grey.shade600)),
          const SizedBox(height: 24),
          // --- 一致性校验 ---
          Card(
            child: Padding(
              padding: const EdgeInsets.all(16),
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Row(
                    children: [
                      Expanded(
                        child: Text('余额与交易一致性校验',
                            style: Theme.of(context).textTheme.titleMedium),
                      ),
                      FilledButton.tonalIcon(
                        onPressed: _checking ? null : _checkConsistency,
                        icon: _checking
                            ? const SizedBox(
                                width: 16,
                                height: 16,
                                child: CircularProgressIndicator(strokeWidth: 2),
                              )
                            : const Icon(Icons.fact_check),
                        label: const Text('校验'),
                      ),
                    ],
                  ),
                  const SizedBox(height: 8),
                  if (_checkError != null)
                    Text(_checkError!, style: TextStyle(color: Colors.red.shade700))
                  else
                    ReconcileDiffTable(
                      discrepancies: _discrepancies ?? const [],
                      onJump: (accountId) {
                        widget.onOpenAccount(accountId);
                      },
                    ),
                ],
              ),
            ),
          ),
          const SizedBox(height: 16),
          // --- 对公打款核对 ---
          Card(
            child: Padding(
              padding: const EdgeInsets.all(16),
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Text('对公打款核对（银行流水 CSV）',
                      style: Theme.of(context).textTheme.titleMedium),
                  const SizedBox(height: 8),
                  TextField(
                    controller: _bankCsvCtrl,
                    maxLines: 6,
                    decoration: const InputDecoration(
                      hintText: '每行：凭证号,金额(分),日期(YYYY-MM-DD)\n如：V20260803001,10000,2026-08-03',
                      border: OutlineInputBorder(),
                    ),
                  ),
                  const SizedBox(height: 8),
                  if (_uploadError != null)
                    Text(_uploadError!, style: TextStyle(color: Colors.red.shade700)),
                  Align(
                    alignment: Alignment.centerRight,
                    child: FilledButton.tonalIcon(
                      onPressed: _uploading ? null : _uploadBank,
                      icon: _uploading
                          ? const SizedBox(
                              width: 16,
                              height: 16,
                              child: CircularProgressIndicator(strokeWidth: 2),
                            )
                          : const Icon(Icons.upload_file),
                      label: const Text('上传并核对'),
                    ),
                  ),
                  if (_bankReport != null) ...[
                    const SizedBox(height: 8),
                    Text('共 ${_bankReport!.total} 行：'
                        '匹配 ${_bankReport!.matched.length} · '
                        '未匹配 ${_bankReport!.unmatched.length}'),
                    if (_bankReport!.unmatched.isNotEmpty)
                      for (final u in _bankReport!.unmatched)
                        ListTile(
                          dense: true,
                          leading: Icon(Icons.error_outline,
                              color: Colors.orange.shade800),
                          title: Text(
                              '${u.row.voucherNo} ¥${(u.row.amount / 100).toStringAsFixed(2)} ${u.row.date}'),
                          subtitle: Text(u.reason),
                        ),
                  ],
                ],
              ),
            ),
          ),
          const SizedBox(height: 16),
          // --- 账单导出 ---
          Card(
            child: Padding(
              padding: const EdgeInsets.all(16),
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Text('账单导出（账户明细）',
                      style: Theme.of(context).textTheme.titleMedium),
                  const SizedBox(height: 8),
                  Form(
                    child: AccountPicker(
                      onSaved: (v) => _statementAccountId = v,
                      labelText: '账户 ID',
                    ),
                  ),
                  const SizedBox(height: 8),
                  Align(
                    alignment: Alignment.centerRight,
                    child: FilledButton.tonalIcon(
                      onPressed: _loadingStatement ? null : _loadStatement,
                      icon: _loadingStatement
                          ? const SizedBox(
                              width: 16,
                              height: 16,
                              child: CircularProgressIndicator(strokeWidth: 2),
                            )
                          : const Icon(Icons.download),
                      label: const Text('导出'),
                    ),
                  ),
                  if (_statementError != null)
                    Text(_statementError!,
                        style: TextStyle(color: Colors.red.shade700)),
                  if (_statement != null) ...[ 
                    const SizedBox(height: 8),
                    const Divider(),
                    Row(
                      mainAxisAlignment: MainAxisAlignment.spaceBetween,
                      children: [
                        Text('期初余额',
                            style: Theme.of(context).textTheme.bodyMedium),
                        MoneyText(_statement!.openingBalance),
                      ],
                    ),
                    Row(
                      mainAxisAlignment: MainAxisAlignment.spaceBetween,
                      children: [
                        Text('期末余额',
                            style: Theme.of(context).textTheme.bodyMedium),
                        MoneyText(_statement!.closingBalance),
                      ],
                    ),
                    const SizedBox(height: 4),
                    Text('明细（${_statement!.entries.length} 笔，生成于 ${_statement!.generatedAt}）',
                        style: Theme.of(context).textTheme.bodySmall),
                    for (final e in _statement!.entries)
                      ListTile(
                        dense: true,
                        contentPadding: EdgeInsets.zero,
                        title: Text(
                            '${e.type}${e.note != null && e.note!.isNotEmpty ? ' · ${e.note}' : ''}'),
                        subtitle: Text('余额 ${MoneyText.format(e.runningBalance)}'),
                        trailing: MoneyText(
                          e.amount,
                          showSign: e.type == 'recharge' || e.type == 'consume',
                        ),
                      ),
                  ],
                ],
              ),
            ),
          ),
        ],
      ),
    );
  }
}

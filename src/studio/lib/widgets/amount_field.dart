import 'package:flutter/material.dart';

/// 金额输入（元）：非负、最多两位小数校验，提交转分。
class AmountField extends StatelessWidget {
  final TextEditingController controller;
  final String labelText;

  const AmountField({
    super.key,
    required this.controller,
    this.labelText = '金额（元）',
  });

  /// 元 → 分（带符号换算，供调用方提交）。
  static int yuanToCents(String yuan) {
    final cleaned = yuan.trim();
    final negative = cleaned.startsWith('-');
    final parts = cleaned.replaceFirst('-', '').split('.');
    final yuanPart = int.parse(parts[0].isEmpty ? '0' : parts[0]);
    final centPart = parts.length > 1 && parts[1].isNotEmpty
        ? int.parse(parts[1].padRight(2, '0').substring(0, 2))
        : 0;
    final cents = yuanPart * 100 + centPart;
    return negative ? -cents : cents;
  }

  @override
  Widget build(BuildContext context) {
    return TextFormField(
      controller: controller,
      keyboardType: const TextInputType.numberWithOptions(decimal: true),
      decoration: InputDecoration(
        labelText: labelText,
        hintText: '如 100.00',
        helperText: '以元输入，提交时转整数分',
        border: const OutlineInputBorder(),
      ),
      validator: (v) {
        final text = v?.trim() ?? '';
        if (text.isEmpty) return '请输入金额';
        if (!RegExp(r'^\d+(\.\d{1,2})?$').hasMatch(text)) {
          return '金额须为非负数字，最多两位小数';
        }
        if (double.parse(text) <= 0) return '金额必须大于 0';
        return null;
      },
    );
  }
}

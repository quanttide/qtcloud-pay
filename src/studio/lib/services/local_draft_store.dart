import 'dart:convert';
import 'dart:io';

import '../models/rule_draft.dart';

/// 本地草稿仓：发放规则草稿存单个 JSON 文件，完全离线，不接触网络。
///
/// 默认路径 `~/.qtcloud-pay-studio/rule_drafts.json`（可用 [filePath] 注入，
/// 测试指到临时目录）。读写失败不抛——草稿是设计期产物，丢了可以重画，
/// 但每次失败都会返回 false 供页面提示。
class LocalDraftStore {
  LocalDraftStore({String? filePath})
      : filePath = filePath ?? _defaultPath();

  final String filePath;

  static String _defaultPath() {
    final home = Platform.environment['HOME'] ??
        Platform.environment['USERPROFILE'] ??
        Directory.current.path;
    return '$home/.qtcloud-pay-studio/rule_drafts.json';
  }

  List<RuleDraft> load() {
    final file = File(filePath);
    if (!file.existsSync()) return [];
    try {
      final data = jsonDecode(file.readAsStringSync());
      final drafts = (data['drafts'] as List? ?? [])
          .map((e) => RuleDraft.fromJson((e as Map).cast<String, dynamic>()))
          .toList();
      return drafts;
    } catch (_) {
      // 文件损坏时不当场覆盖，让页面提示并由用户决定。
      return [];
    }
  }

  bool save(List<RuleDraft> drafts) {
    try {
      final file = File(filePath);
      file.parent.createSync(recursive: true);
      file.writeAsStringSync('${encodeDraftSet(drafts)}\n');
      return true;
    } catch (_) {
      return false;
    }
  }
}

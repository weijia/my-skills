import 'dart:async';
import 'package:flutter/material.dart';
import 'package:appflowy_editor/appflowy_editor.dart';

/// AppFlowy Editor 基础集成模板
/// 包含：工具栏、块类型切换、表格操作、Markdown 互转
class AppFlowyNoteEditor extends StatefulWidget {
  final String initialMarkdown;
  final ValueChanged<String>? onContentChanged;

  const AppFlowyNoteEditor({
    super.key,
    required this.initialMarkdown,
    this.onContentChanged,
  });

  @override
  State<AppFlowyNoteEditor> createState() => _AppFlowyNoteEditorState();
}

class _AppFlowyNoteEditorState extends State<AppFlowyNoteEditor> {
  late EditorState _editorState;
  StreamSubscription? _transactionSub;
  StreamSubscription? _selectionSub;
  bool _isInTable = false;

  @override
  void initState() {
    super.initState();
    final document = markdownToDocument(widget.initialMarkdown);
    _editorState = EditorState(document: document);

    _transactionSub = _editorState.transactionStream.listen((_) {
      widget.onContentChanged?.call(documentToMarkdown(_editorState.document));
    });

    // 监听选区变化，更新工具栏状态
    _selectionSub = _editorState.transactionStream.listen((_) {
      _updateTableState();
    });
  }

  @override
  void dispose() {
    _transactionSub?.cancel();
    _selectionSub?.cancel();
    super.dispose();
  }

  void _updateTableState() {
    final wasInTable = _isInTable;
    _isInTable = _isSelectionInTable();
    if (wasInTable != _isInTable) {
      setState(() {});
    }
  }

  bool _isSelectionInTable() {
    final sel = _editorState.selection;
    if (sel == null) return false;
    for (int i = sel.start.path.length - 1; i >= 0; i--) {
      final path = sel.start.path.sublist(0, i + 1);
      final node = _editorState.getNodeAtPath(path);
      if (node != null && node.type == TableBlockKeys.type) return true;
    }
    return false;
  }

  Node? _findTableNode() {
    final sel = _editorState.selection;
    if (sel == null) return null;
    for (int i = sel.start.path.length - 1; i >= 0; i--) {
      final path = sel.start.path.sublist(0, i + 1);
      final node = _editorState.getNodeAtPath(path);
      if (node != null && node.type == TableBlockKeys.type) return node;
    }
    return null;
  }

  MapEntry<int, int>? _getTableCellPosition() {
    final sel = _editorState.selection;
    if (sel == null) return null;
    if (sel.start.path.length < 3) return null;
    final colIndex = sel.start.path[sel.start.path.length - 2];
    final rowIndex = sel.start.path[sel.start.path.length - 1];
    return MapEntry(rowIndex, colIndex);
  }

  // ==================== 块类型切换 ====================

  void _toggleBlockType(String targetType) {
    final selection = _editorState.selection;
    if (selection == null) return;
    final node = _editorState.getNodeAtPath(selection.start.path);
    if (node == null) return;
    final newType = node.type == targetType ? ParagraphBlockKeys.type : targetType;
    _editorState.formatNode(selection, (node) => node.copyWith(type: newType));
  }

  void _toggleTodoList() {
    final selection = _editorState.selection;
    if (selection == null) return;
    final node = _editorState.getNodeAtPath(selection.start.path);
    if (node == null) return;
    final isTodo = node.type == TodoListBlockKeys.type;
    final newType = isTodo ? ParagraphBlockKeys.type : TodoListBlockKeys.type;
    if (isTodo) {
      _editorState.formatNode(selection, (node) => node.copyWith(type: newType));
    } else {
      _editorState.formatNode(selection, (node) => node.copyWith(
        type: newType,
        attributes: {...node.attributes, TodoListBlockKeys.checked: false},
      ));
    }
  }

  void _toggleHeading(int level) {
    final selection = _editorState.selection;
    if (selection == null) return;
    final node = _editorState.getNodeAtPath(selection.start.path);
    if (node == null) return;
    final isHeading = node.type == HeadingBlockKeys.type;
    final currentLevel = node.attributes[HeadingBlockKeys.level] ?? 1;
    final shouldToggleOff = isHeading && currentLevel == level;
    _editorState.formatNode(selection, (node) => node.copyWith(
      type: shouldToggleOff ? ParagraphBlockKeys.type : HeadingBlockKeys.type,
      attributes: shouldToggleOff
          ? <String, dynamic>{}
          : {...node.attributes, HeadingBlockKeys.level: level},
    ));
  }

  // ==================== 表格操作 ====================

  void _insertTable(int rows, int cols) {
    final sel = _editorState.selection;
    final lastPath = [_editorState.document.root.children.length - 1];
    final insertPath = sel?.end.path ?? lastPath;
    final tableNode = TableNode.fromList(
      List.generate(cols, (_) => List.generate(rows, (_) => '')),
    );
    final transaction = _editorState.transaction;
    final currentNode = _editorState.getNodeAtPath(insertPath);
    if (currentNode != null && currentNode.delta != null && currentNode.delta!.isEmpty) {
      transaction.deleteNode(currentNode);
      transaction.insertNode(insertPath, tableNode.node);
    } else {
      transaction.insertNode(insertPath.next, tableNode.node);
    }
    transaction.afterSelection = Selection.collapsed(
      Position(path: insertPath + [0, 0], offset: 0),
    );
    _editorState.apply(transaction);
  }

  void _tableAddRow() {
    final tableNode = _findTableNode();
    final cellPos = _getTableCellPosition();
    if (tableNode == null || cellPos == null) return;
    TableActions.add(tableNode, cellPos.key, _editorState, TableDirection.row);
  }

  void _tableAddColumn() {
    final tableNode = _findTableNode();
    final cellPos = _getTableCellPosition();
    if (tableNode == null || cellPos == null) return;
    TableActions.add(tableNode, cellPos.value, _editorState, TableDirection.col);
  }

  void _tableDeleteRow() {
    final tableNode = _findTableNode();
    final cellPos = _getTableCellPosition();
    if (tableNode == null || cellPos == null) return;
    if (TableNode(node: tableNode).rowsLen <= 1) return;
    TableActions.delete(tableNode, cellPos.key, _editorState, TableDirection.row);
  }

  void _tableDeleteColumn() {
    final tableNode = _findTableNode();
    final cellPos = _getTableCellPosition();
    if (tableNode == null || cellPos == null) return;
    if (TableNode(node: tableNode).colsLen <= 1) return;
    TableActions.delete(tableNode, cellPos.value, _editorState, TableDirection.col);
  }

  // ==================== 构建 UI ====================

  @override
  Widget build(BuildContext context) {
    final colorScheme = Theme.of(context).colorScheme;
    return Column(
      children: [
        _buildToolbar(colorScheme),
        const Divider(height: 1),
        Expanded(child: _buildEditor(colorScheme)),
      ],
    );
  }

  Widget _buildEditor(ColorScheme colorScheme) {
    return AppFlowyEditor(
      editorState: _editorState,
      editable: true,
      autoFocus: true,
      editorStyle: EditorStyle.desktop(
        padding: const EdgeInsets.all(16),
        cursorColor: colorScheme.primary,
        selectionColor: colorScheme.primaryContainer.withValues(alpha: 0.4),
        textStyleConfiguration: TextStyleConfiguration(
          text: TextStyle(color: colorScheme.onSurface, fontSize: 16, height: 1.5),
        ),
      ),
      blockComponentBuilders: standardBlockComponentBuilderMap,
      characterShortcutEvents: standardCharacterShortcutEvents,
      commandShortcutEvents: standardCommandShortcutEvents,
    );
  }

  Widget _buildToolbar(ColorScheme colorScheme) {
    return Material(
      color: colorScheme.surfaceContainerLow,
      child: SingleChildScrollView(
        scrollDirection: Axis.horizontal,
        padding: const EdgeInsets.symmetric(horizontal: 8, vertical: 4),
        child: Row(
          children: _isInTable
              ? _buildTableToolbar(colorScheme)
              : _buildNormalToolbar(colorScheme),
        ),
      ),
    );
  }

  List<Widget> _buildNormalToolbar(ColorScheme colorScheme) {
    return [
      _btn(Icons.title, 'H1', () => _toggleHeading(1)),
      _btn(Icons.format_size, 'H2', () => _toggleHeading(2)),
      _divider(colorScheme),
      _btn(Icons.format_bold, 'B', () => _editorState.toggleAttribute(BuiltInAttributeKey.bold)),
      _btn(Icons.format_italic, 'I', () => _editorState.toggleAttribute(BuiltInAttributeKey.italic)),
      _btn(Icons.format_underlined, 'U', () => _editorState.toggleAttribute(BuiltInAttributeKey.underline)),
      _divider(colorScheme),
      _btn(Icons.format_list_bulleted, 'Bullet', () => _toggleBlockType(BulletedListBlockKeys.type)),
      _btn(Icons.format_list_numbered, 'Number', () => _toggleBlockType(NumberedListBlockKeys.type)),
      _btn(Icons.check_box_outlined, 'Todo', _toggleTodoList),
      _divider(colorScheme),
      _btn(Icons.format_quote, 'Quote', () => _toggleBlockType(QuoteBlockKeys.type)),
      _btn(Icons.table_chart, 'Table', () => _showInsertTableDialog()),
    ];
  }

  List<Widget> _buildTableToolbar(ColorScheme colorScheme) {
    return [
      _btn(Icons.add, 'Add Row', _tableAddRow),
      _btn(Icons.view_column, 'Add Col', _tableAddColumn),
      _divider(colorScheme),
      _btn(Icons.delete_outline, 'Del Row', _tableDeleteRow),
      _btn(Icons.delete_sweep, 'Del Col', _tableDeleteColumn),
      _divider(colorScheme),
      _btn(Icons.content_copy, 'Copy Row', () {
        final tableNode = _findTableNode();
        final cellPos = _getTableCellPosition();
        if (tableNode == null || cellPos == null) return;
        TableActions.duplicate(tableNode, cellPos.key, _editorState, TableDirection.row);
      }),
    ];
  }

  Widget _btn(IconData icon, String tooltip, VoidCallback onPressed) {
    return IconButton(
      icon: Icon(icon, size: 20),
      tooltip: tooltip,
      onPressed: onPressed,
      padding: const EdgeInsets.all(6),
      constraints: const BoxConstraints(minWidth: 36, minHeight: 36),
    );
  }

  Widget _divider(ColorScheme colorScheme) {
    return Container(
      width: 1, height: 24,
      margin: const EdgeInsets.symmetric(horizontal: 4),
      color: colorScheme.outlineVariant.withValues(alpha: 0.5),
    );
  }

  void _showInsertTableDialog() {
    showDialog(
      context: context,
      builder: (context) => _InsertTableDialog(onInsert: _insertTable),
    );
  }
}

class _InsertTableDialog extends StatefulWidget {
  final Function(int rows, int cols) onInsert;
  const _InsertTableDialog({required this.onInsert});

  @override
  State<_InsertTableDialog> createState() => _InsertTableDialogState();
}

class _InsertTableDialogState extends State<_InsertTableDialog> {
  int _rows = 3;
  int _cols = 3;

  @override
  Widget build(BuildContext context) {
    return AlertDialog(
      title: const Text('Insert Table'),
      content: Column(
        mainAxisSize: MainAxisSize.min,
        children: [
          Row(children: [
            const Text('Rows:'), const SizedBox(width: 16),
            Expanded(child: Slider(value: _rows.toDouble(), min: 1, max: 10, divisions: 9,
              label: _rows.toString(),
              onChanged: (v) => setState(() => _rows = v.round()))),
            Text('$_rows'),
          ]),
          Row(children: [
            const Text('Cols:'), const SizedBox(width: 16),
            Expanded(child: Slider(value: _cols.toDouble(), min: 1, max: 10, divisions: 9,
              label: _cols.toString(),
              onChanged: (v) => setState(() => _cols = v.round()))),
            Text('$_cols'),
          ]),
        ],
      ),
      actions: [
        TextButton(onPressed: () => Navigator.pop(context), child: const Text('Cancel')),
        FilledButton(onPressed: () {
          widget.onInsert(_rows, _cols);
          Navigator.pop(context);
        }, child: const Text('Insert')),
      ],
    );
  }
}

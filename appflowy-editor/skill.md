# Skill: AppFlowy Editor～Flutter 富文本编辑器集成

> 在 Flutter 项目中集成 AppFlowy Editor 作为 WYSIWYG 编辑器，支持表格、列表、标题等富文本功能。

---

## 概述

本 skill 用于在 Flutter 项目中集成 `appflowy_editor` 包，替代原生 TextField 实现所见即所得的 Markdown 编辑。

| 属性 | 值 |
|------|-----|
| 包名 | `appflowy_editor` |
| 语言 | Dart / Flutter |
| 适用场景 | 笔记应用、文档编辑、Markdown 编辑器 |

**关键约束**：
- `toggleAttribute` 只适用于**内联属性**（bold、italic 等），**不适用于块类型**
- 块类型切换必须使用 `editorState.formatNode()`
- `selectionStream` 在某些版本中不存在，推荐用 `transactionStream` 监听选区变化
- `withOpacity` 已废弃，必须使用 `withValues(alpha: x)`
- `copyWith` 在某些 Node 类型上已废弃

---

## 前置条件

- Flutter 项目（Android / iOS）
- Dart null safety 已启用
- `appflowy_editor` 已添加到 `pubspec.yaml`

```yaml
dependencies:
  appflowy_editor: ^1.0.0
```

---

## 步骤一：基本集成

参考 [`templates/editor_basic.dart`](./templates/editor_basic.dart) 创建编辑器 Widget。

核心结构：

```dart
import 'package:appflowy_editor/appflowy_editor.dart';

// 1. 创建 EditorState
final document = markdownToDocument(markdownString);
final editorState = EditorState(document: document);

// 2. 渲染编辑器
AppFlowyEditor(
  editorState: editorState,
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
)
```

---

## 步骤二：内联属性切换（Bold / Italic / Underline）

✅ **正确**：使用 `toggleAttribute`

```dart
_editorState.toggleAttribute(BuiltInAttributeKey.bold);
_editorState.toggleAttribute(BuiltInAttributeKey.italic);
_editorState.toggleAttribute(BuiltInAttributeKey.underline);
_editorState.toggleAttribute(BuiltInAttributeKey.strikethrough);
```

这些 API 可以直接调用，会作用于当前选区或光标所在位置。

---

## 步骤三：块类型切换（Bullet List / Numbered List / Checkbox / Quote）

❌ **错误**：`toggleAttribute` 对块类型无效！

```dart
// ❌ 这样写不会生效
_editorState.toggleAttribute(BuiltInAttributeKey.bulletedList);
```

✅ **正确**：使用 `formatNode` 改变 Node 的 `type`

```dart
void _toggleBlockType(String targetType) {
  final selection = _editorState.selection;
  if (selection == null) return;

  final node = _editorState.getNodeAtPath(selection.start.path);
  if (node == null) return;

  // 如果已经是目标类型，切换回段落；否则切换为目标类型
  final newType = node.type == targetType
      ? ParagraphBlockKeys.type
      : targetType;

  _editorState.formatNode(
    selection,
    (node) => node.copyWith(type: newType),
  );
}
```

### 块类型常量速查

| 功能 | 常量 | type 值 |
|------|------|---------|
| 段落 | `ParagraphBlockKeys.type` | `'paragraph'` |
| 无序列表 | `BulletedListBlockKeys.type` | `'bulleted_list'` |
| 有序列表 | `NumberedListBlockKeys.type` | `'numbered_list'` |
| 待办事项 | `TodoListBlockKeys.type` | `'todo_list'` |
| 引用 | `QuoteBlockKeys.type` | `'quote'` |
| 标题 | `HeadingBlockKeys.type` | `'heading'` |
| 代码块 | `BuiltInAttributeKey.code` | `'code_block'` |

### Todo List 特殊处理

Todo List 需要额外的 `checked` 属性：

```dart
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
      attributes: {
        ...node.attributes,
        TodoListBlockKeys.checked: false,
      },
    ));
  }
}
```

### Heading 特殊处理

Heading 需要设置 `level` 属性：

```dart
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
```

---

## 步骤四：表格操作

### 插入表格

```dart
void _insertTable(int rows, int cols) {
  final tableData = List.generate(
    cols,
    (_) => List.generate(rows, (_) => ''),
  );
  final tableNode = TableNode.fromList(tableData);

  final transaction = _editorState.transaction;
  final insertPath = [_editorState.document.root.children.length - 1];
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
```

### 表格行列操作

```dart
// 添加行
TableActions.add(tableNode, rowIndex, _editorState, TableDirection.row);

// 添加列
TableActions.add(tableNode, colIndex, _editorState, TableDirection.col);

// 删除行（至少保留一行）
TableActions.delete(tableNode, rowIndex, _editorState, TableDirection.row);

// 删除列（至少保留一列）
TableActions.delete(tableNode, colIndex, _editorState, TableDirection.col);

// 复制行
TableActions.duplicate(tableNode, rowIndex, _editorState, TableDirection.row);
```

### 检测光标是否在表格内

```dart
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
```

### 获取表格中的行列位置

```dart
MapEntry<int, int>? _getTableCellPosition() {
  final sel = _editorState.selection;
  if (sel == null) return null;
  // Path 结构: [..., tableIndex, colIndex, rowIndex]
  if (sel.start.path.length < 3) return null;
  final colIndex = sel.start.path[sel.start.path.length - 2];
  final rowIndex = sel.start.path[sel.start.path.length - 1];
  return MapEntry(rowIndex, colIndex);
}
```

---

## 步骤五：工具栏动态切换

当光标在表格内时显示表格工具栏，否则显示格式化工具栏。

### ❌ 常见错误：工具栏不刷新

```dart
// ❌ 只在 build 时检查一次，光标移动后不会更新
Widget _buildToolbar() {
  final isInTable = _isSelectionInTable();
  return isInTable ? _buildTableToolbar() : _buildNormalToolbar();
}
```

### ✅ 正确：监听选区变化触发 rebuild

```dart
StreamSubscription? _selectionSub;

@override
void initState() {
  super.initState();
  // 用 transactionStream 监听（selectionStream 在某些版本不存在）
  _selectionSub = _editorState.transactionStream.listen((_) {
    _updateTableState();
  });
}

void _updateTableState() {
  final wasInTable = _isInTable;
  _isInTable = _isSelectionInTable();
  if (wasInTable != _isInTable) {
    setState(() {}); // 触发 rebuild
  }
}

@override
void dispose() {
  _selectionSub?.cancel();
  super.dispose();
}
```

---

## 步骤六：Markdown 转换

```dart
// Markdown → Document
final document = markdownToDocument(markdownString);

// Document → Markdown
final markdown = documentToMarkdown(editorState.document);
```

---

## 踩坑记录

### 1. `CodeBlockKeys` 不存在

❌ `CodeBlockKeys.type` 在某些版本中不存在。

✅ 使用 `BuiltInAttributeKey.code` 或直接用字符串 `'code_block'`。

### 2. `withOpacity` 已废弃

❌ `color.withOpacity(0.4)`

✅ `color.withValues(alpha: 0.4)`

### 3. `selectionStream` 不存在

❌ `_editorState.selectionStream.listen(...)`

✅ 使用 `_editorState.transactionStream.listen(...)` 代替，在回调中检查 `_editorState.selection`。

### 4. Node `copyWith` 废弃

在某些版本中，`node.copyWith(type: ...)` 可能被标记为废弃。如果遇到编译错误，改用直接操作 `node.type` 或查看当前版本的 API 文档。

### 5. 无选择时块类型切换

`formatNode` 需要有效的 `selection`。如果 `_editorState.selection` 为 null（例如编辑器未获得焦点），操作不会生效。确保在调用前检查 selection 不为 null。

### 6. 表格工具栏不切换

工具栏的 `_buildToolbar` 只在 `build` 时执行一次。光标从普通文本移到表格内时，如果没有触发 `setState`，工具栏不会更新。必须通过 `transactionStream` 监听选区变化并调用 `setState`。

### 7. Debug APK 签名不一致导致无法覆盖安装

每次 CI 构建的 debug APK 使用自动生成的签名，导致无法覆盖安装。解决方案：

1. 生成固定 keystore：`keytool -genkeypair -keystore debug-keystore.jks -alias my-debug -validity 36500`
2. 将 keystore base64 编码存为 GitHub Secret
3. CI 构建时解码 keystore 并配置 `build.gradle` 使用固定签名

详见 [`templates/android_signing.gradle`](./templates/android_signing.gradle)。

---

## 新项目快速配置清单

1. **添加依赖** → `pubspec.yaml` 添加 `appflowy_editor: ^1.0.0`
2. **创建编辑器 Widget** → 参考 `templates/editor_basic.dart`
3. **实现工具栏** → 内联属性用 `toggleAttribute`，块类型用 `formatNode`
4. **表格支持** → 用 `TableNode.fromList()` 插入，`TableActions` 操作行列
5. **工具栏切换** → 监听 `transactionStream`，检测 `_isSelectionInTable()`
6. **Markdown 互转** → `markdownToDocument()` / `documentToMarkdown()`
7. **固定签名** → 配置 debug keystore 确保 APK 可覆盖安装

---

## 参考链接

- [AppFlowy Editor 官方文档](https://docs.appflowy.io/docs/appflowy/product/editor)
- [appflowy_editor pub.dev](https://pub.dev/packages/appflowy_editor)
- [AppFlowy Editor GitHub](https://github.com/AppFlowy-IO/appflowy-editor)

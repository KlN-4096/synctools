/*
文件作用:
- 提供通用的表格模型实现
- 简化表格数据绑定
- 支持自定义数据源和列定义
*/

package shared

import (
	"github.com/lxn/walk"
)

// TableColumn 表格列定义
type TableColumn struct {
	Title string                            // 列标题
	Width int                               // 列宽度
	Value func(row interface{}) interface{} // 获取单元格值的函数
}

// TableModel 通用表格模型
type TableModel struct {
	walk.TableModelBase
	columns []TableColumn                       // 列定义
	rows    []interface{}                       // 数据源
	filter  string                              // 过滤条件
	onSort  func(col int, order walk.SortOrder) // 排序回调
}

// NewTableModel 创建新的表格模型
func NewTableModel(columns []TableColumn) *TableModel {
	return &TableModel{
		columns: columns,
		rows:    make([]interface{}, 0),
	}
}

// RowCount 返回行数
func (m *TableModel) RowCount() int {
	return len(m.rows)
}

// ColumnCount 返回列数
func (m *TableModel) ColumnCount() int {
	return len(m.columns)
}

// ColumnTitle 返回列标题
func (m *TableModel) ColumnTitle(col int) string {
	if col < 0 || col >= len(m.columns) {
		return ""
	}
	return m.columns[col].Title
}

// Value 返回单元格值
func (m *TableModel) Value(row, col int) interface{} {
	if row < 0 || row >= len(m.rows) || col < 0 || col >= len(m.columns) {
		return nil
	}
	return m.columns[col].Value(m.rows[row])
}

// Sort 排序
func (m *TableModel) Sort(col int, order walk.SortOrder) error {
	if m.onSort != nil {
		m.onSort(col, order)
	}
	return nil
}

// SetRows 设置数据源
func (m *TableModel) SetRows(rows []interface{}) {
	m.rows = rows
	m.PublishRowsReset()
}

// SetSortCallback 设置排序回调
func (m *TableModel) SetSortCallback(callback func(col int, order walk.SortOrder)) {
	m.onSort = callback
}

// PublishRowsReset 通知行重置
func (m *TableModel) PublishRowsReset() {
	m.TableModelBase.PublishRowsReset()
}

/*
文件作用:
- 提供通用的表格模型实现
- 简化表格数据绑定
- 支持自定义数据源和列定义
*/

package shared

import (
	"sort"
	"strings"

	"github.com/lxn/walk"

	"synctools/codes/internal/interfaces"
)

// TableColumn 表格列定义
type TableColumn struct {
	Title string                            // 列标题
	Width int                               // 列宽度
	Value func(row interface{}) interface{} // 获取单元格值的函数
}

// TableModel 通用表格模型
type TableModel struct {
	walk.ReflectTableModelBase
	columns []TableColumn // 列定义
	rows    []interface{} // 数据源
	cache   []interface{} // 缓存数据
	filter  string        // 过滤条件

	// 排序
	sortColumn int
	sortOrder  walk.SortOrder

	// 服务和日志
	service interfaces.SyncService // 数据服务
	logger  interfaces.Logger      // 日志接口

	// 数据源绑定
	dataSource    func() []interface{}   // 数据源获取函数
	filterSource  func(interface{}) bool // 数据过滤函数
	compareSource func(i, j int) bool    // 数据比较函数
	onUpdate      func()                 // 数据更新回调
}

// NewTableModel 创建新的表格模型
func NewTableModel(columns []TableColumn, service interfaces.SyncService, logger interfaces.Logger) *TableModel {
	return &TableModel{
		columns: columns,
		rows:    make([]interface{}, 0),
		cache:   make([]interface{}, 0),
		service: service,
		logger:  logger,
	}
}

// SetDataSource 设置数据源
func (m *TableModel) SetDataSource(source func() []interface{}) {
	m.dataSource = source
	m.RefreshCache()
}

// SetFilterSource 设置过滤函数
func (m *TableModel) SetFilterSource(filter func(interface{}) bool) {
	m.filterSource = filter
	m.Filter(m.filter)
}

// SetCompareSource 设置比较函数
func (m *TableModel) SetCompareSource(compare func(i, j int) bool) {
	m.compareSource = compare
}

// SetUpdateCallback 设置更新回调
func (m *TableModel) SetUpdateCallback(callback func()) {
	m.onUpdate = callback
}

// RowCount 返回行数
func (m *TableModel) RowCount() int {
	count := len(m.rows)
	if m.logger != nil {
		m.logger.Debug("获取行数", interfaces.Fields{
			"count": count,
		})
	}
	return count
}

// Value 返回单元格值
func (m *TableModel) Value(row, col int) interface{} {
	if row < 0 || row >= len(m.rows) || col < 0 || col >= len(m.columns) {
		if m.logger != nil {
			m.logger.Debug("Value: 无效的索引", interfaces.Fields{
				"row":     row,
				"col":     col,
				"max_row": len(m.rows),
				"max_col": len(m.columns),
			})
		}
		return nil
	}

	value := m.columns[col].Value(m.rows[row])
	if m.logger != nil {
		m.logger.Debug("获取单元格值", interfaces.Fields{
			"row":   row,
			"col":   col,
			"value": value,
		})
	}
	return value
}

// Sort 排序
func (m *TableModel) Sort(col int, order walk.SortOrder) error {
	if m.logger != nil {
		m.logger.Debug("开始排序", interfaces.Fields{
			"column": col,
			"order":  order,
		})
	}

	m.sortColumn = col
	m.sortOrder = order

	if m.compareSource != nil {
		sort.SliceStable(m.rows, m.compareSource)
	} else {
		sort.SliceStable(m.rows, func(i, j int) bool {
			a := m.columns[col].Value(m.rows[i])
			b := m.columns[col].Value(m.rows[j])

			if m.sortOrder == walk.SortAscending {
				return less(a, b)
			}
			return less(b, a)
		})
	}

	m.PublishRowsReset()
	return nil
}

// less 比较两个值的大小
func less(a, b interface{}) bool {
	if a == nil {
		return b != nil
	}
	if b == nil {
		return false
	}

	switch av := a.(type) {
	case string:
		if bv, ok := b.(string); ok {
			return av < bv
		}
	case int:
		if bv, ok := b.(int); ok {
			return av < bv
		}
	case float64:
		if bv, ok := b.(float64); ok {
			return av < bv
		}
	case bool:
		if bv, ok := b.(bool); ok {
			return !av && bv
		}
	}

	return false
}

// SetRows 设置数据源
func (m *TableModel) SetRows(rows []interface{}) {
	if m.logger != nil {
		m.logger.Debug("设置数据源", interfaces.Fields{
			"count": len(rows),
		})
	}

	m.rows = rows
	m.cache = make([]interface{}, len(rows))
	copy(m.cache, rows)
	m.PublishRowsReset()

	if m.onUpdate != nil {
		m.onUpdate()
	}
}

// RefreshCache 刷新缓存
func (m *TableModel) RefreshCache() {
	if m.logger != nil {
		m.logger.Debug("开始刷新缓存", interfaces.Fields{
			"before_count": len(m.cache),
		})
	}

	if m.dataSource != nil {
		m.SetRows(m.dataSource())
	} else {
		m.rows = make([]interface{}, len(m.cache))
		copy(m.rows, m.cache)
		m.PublishRowsReset()
	}

	if m.logger != nil {
		m.logger.Debug("缓存刷新完成", interfaces.Fields{
			"after_count": len(m.rows),
		})
	}
}

// Filter 过滤数据
func (m *TableModel) Filter(filter string) {
	if m.logger != nil {
		m.logger.Debug("开始过滤数据", interfaces.Fields{
			"filter": filter,
		})
	}

	m.filter = filter
	if filter == "" {
		m.rows = make([]interface{}, len(m.cache))
		copy(m.rows, m.cache)
	} else {
		m.rows = make([]interface{}, 0)
		for _, row := range m.cache {
			if m.filterSource != nil {
				if m.filterSource(row) {
					m.rows = append(m.rows, row)
				}
			} else {
				for _, col := range m.columns {
					if val := col.Value(row); val != nil {
						if s, ok := val.(string); ok {
							if s != "" && containsIgnoreCase(s, filter) {
								m.rows = append(m.rows, row)
								break
							}
						}
					}
				}
			}
		}
	}

	if m.logger != nil {
		m.logger.Debug("过滤完成", interfaces.Fields{
			"filtered_count": len(m.rows),
		})
	}

	m.PublishRowsReset()
}

// containsIgnoreCase 忽略大小写的字符串包含判断
func containsIgnoreCase(s, substr string) bool {
	s = strings.ToLower(s)
	substr = strings.ToLower(substr)
	return strings.Contains(s, substr)
}

// Columns 返回列定义
func (m *TableModel) Columns() []*walk.TableViewColumn {
	cols := make([]*walk.TableViewColumn, len(m.columns))
	for i, col := range m.columns {
		column := new(walk.TableViewColumn)
		column.SetTitle(col.Title)
		column.SetWidth(col.Width)
		cols[i] = column
	}
	return cols
}

// GetRow 获取指定行的数据
func (m *TableModel) GetRow(row int) interface{} {
	if row < 0 || row >= len(m.rows) {
		if m.logger != nil {
			m.logger.Debug("GetRow: 无效的行索引", interfaces.Fields{
				"row":     row,
				"max_row": len(m.rows),
			})
		}
		return nil
	}
	return m.rows[row]
}

// GetSortInfo 获取当前排序信息
func (m *TableModel) GetSortInfo() (column int, order walk.SortOrder) {
	return m.sortColumn, m.sortOrder
}

// GetFilter 获取当前过滤条件
func (m *TableModel) GetFilter() string {
	return m.filter
}

// GetRows 获取当前显示的所有行数据
func (m *TableModel) GetRows() []interface{} {
	return m.rows
}

// GetCache 获取缓存的所有行数据
func (m *TableModel) GetCache() []interface{} {
	return m.cache
}

package interfaces

// Storage 定义存储接口
type Storage interface {
	// Save 保存数据
	Save(key string, data interface{}) error

	// Load 加载数据到指定对象
	Load(key string, data interface{}) error

	// Delete 删除数据
	Delete(key string) error

	// Exists 检查数据是否存在
	Exists(key string) bool

	// List 列出所有键
	List() ([]string, error)
}

package connector

import (
    "fmt"
    "sync"
)

// Registry 用于保存所有 Connector 的构造器，实现自动注册
var (
    registryMu sync.Mutex
    registry   = make(map[string]func() Connector)
)

// Register 由各个 Connector 在其 init() 中调用
func Register(kind string, constructor func() Connector) {
    registryMu.Lock()
    defer registryMu.Unlock()
    if _, ok := registry[kind]; ok {
        panic(fmt.Sprintf("connector %q already registered", kind))
    }
    registry[kind] = constructor
}

// GetConnector 根据数据库类型获取对应的 Connector 实例
func GetConnector(kind string) (Connector, error) {
    registryMu.Lock()
    constructor, ok := registry[kind]
    registryMu.Unlock()
    if !ok {
        return nil, fmt.Errorf("no connector for %q", kind)
    }
    return constructor(), nil
}
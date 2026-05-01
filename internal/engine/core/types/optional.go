package types

// Optional 表示一个可能未设置的值，用于替代 Value + HasValue 布尔对模式。
type Optional[T any] struct {
	value T
	set   bool
}

// Some 创建一个已设置的 Optional 值。
func Some[T any](v T) Optional[T] {
	return Optional[T]{value: v, set: true}
}

// None 创建一个未设置的 Optional 值。
func None[T any]() Optional[T] {
	return Optional[T]{}
}

// IsSet 返回值是否已被显式设置。
func (o Optional[T]) IsSet() bool {
	return o.set
}

// Get 返回值。如果未设置，返回零值。
func (o Optional[T]) Get() T {
	return o.value
}

// OrDefault 返回已设置的值，未设置时返回默认值。
func (o Optional[T]) OrDefault(d T) T {
	if o.set {
		return o.value
	}
	return d
}

package values

type Orderedmapper interface {
	Get(key interface{}) (interface{}, bool)
	Len() int
	Range(f func(key, value interface{}) bool)
}

type orederedmapValue struct {
	om Orderedmapper
	valueEmbed
}

func (v orederedmapValue) Interface() interface{} { return v.om }

func (v orederedmapValue) Contains(elem Value) bool {
	e := elem.Interface()
	_, ok := v.om.Get(e)
	return ok
}

func (v orederedmapValue) IndexValue(iv Value) Value {
	elm, ok := v.om.Get(iv.Interface())
	if !ok {
		return nilValue
	}
	return ValueOf(elm)
}

func (v orederedmapValue) PropertyValue(iv Value) Value {
	ivi := iv.Interface()
	elm, ok := v.om.Get(ivi)
	if !ok {
		if ivi == sizeKey {
			return ValueOf(v.om.Len())
		}
		return nilValue
	}
	return ValueOf(elm)
}

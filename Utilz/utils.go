package Holly
import (
	"reflect"
	"log"
)



type Object interface{}

func fmap(xs []Object, f func(Object) Object) []Object {
	ys := make([]Object, len(xs))
	for i, x := range xs {
		ys[i] = f(x)
	}
	return ys
}


func flatMap(xs []Object, f func(Object) []Object) []Object {
	var ys []Object
	for _, x := range xs {
		tmp := f(x)
		//		fmt.Println(tmp)
		for _, val := range tmp {
			ys = append(ys, val)
		}
	}
	return ys
}


func filter(xs []Object, f func(Object) bool) []Object {
	var ys []Object
	for _, x := range xs {
		if f(x) {
			ys = append(ys, x)
		}
	}
	return ys
}

func TypedMap(f interface{}, xs interface{}) interface{} {
	vf := reflect.ValueOf(f)
	vxs := reflect.ValueOf(xs)

	ftype := vf.Type()
	xstype := vxs.Type()

	// 1) Map's first parameter type must be `func(A) B`
	if ftype.Kind() != reflect.Func {
		log.Panicf("`f` should be %s but got %s", reflect.Func, ftype.Kind())
	}
	if ftype.NumIn() != 1 {
		log.Panicf("`f` should have 1 parameter but it has %d parameters",
			ftype.NumIn())
	}
	if ftype.NumOut() != 1 {
		log.Panicf("`f` should return 1 value but it returns %d values",
			ftype.NumOut())
	}

	// 2) Map's second parameter type must be `[]A1` where `A == A1`.
	if xstype.Kind() != reflect.Slice {
		log.Panicf("`xs` should be %s but got %s", reflect.Slice, xstype.Kind())
	}
	if xstype.Elem() != ftype.In(0) {
		log.Panicf("type of `f`'s parameter should be %s but xs contains %s",
			ftype.In(0), xstype.Elem())
	}

	// 3) Map's return type must be `[]B1` where `B == B1`.
	tys := reflect.SliceOf(vf.Type().Out(0))

	vys := reflect.MakeSlice(tys, vxs.Len(), vxs.Len())
	for i := 0; i < vxs.Len(); i++ {
		y := vf.Call([]reflect.Value{vxs.Index(i)})[0]
		vys.Index(i).Set(y)
	}
	return vys.Interface()
}
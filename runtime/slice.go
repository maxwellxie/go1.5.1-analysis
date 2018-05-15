// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package runtime

import (
	"unsafe"
)

// slice底层的数据结构
// 包含一个指向底层数组首地址的指针
// 一个表示slice长度的len
// 一个表示slice容量的cap
type slice struct {
	array unsafe.Pointer
	len   int
	cap   int
}

// 新建slice, 创建的语法形如 make([]T, len, cap)
// newarray函数是指定长度的数组内存
func makeslice(t *slicetype, len64, cap64 int64) slice {
	len := int(len64)
	if len64 < 0 || int64(len) != len64 || t.elem.size > 0 && uintptr(len) > _MaxMem/uintptr(t.elem.size) {
		panic(errorString("makeslice: len out of range"))
	}
	cap := int(cap64)
	if cap < len || int64(cap) != cap64 || t.elem.size > 0 && uintptr(cap) > _MaxMem/uintptr(t.elem.size) {
		panic(errorString("makeslice: cap out of range"))
	}
	p := newarray(t.elem, uintptr(cap))
	return slice{p, len, cap}
}

// 用在 append(slice, slice...)
func growslice_n(t *slicetype, old slice, n int) slice {
	if n < 1 {
		panic(errorString("growslice: invalid n"))
	}
	return growslice(t, old, old.cap+n)
}

// slice的动态扩容
func growslice(t *slicetype, old slice, cap int) slice {
	if cap < old.cap || t.elem.size > 0 && uintptr(cap) > _MaxMem/uintptr(t.elem.size) {
		panic(errorString("growslice: cap out of range"))
	}

	et := t.elem
	if et.size == 0 {
		return slice{unsafe.Pointer(&zerobase), old.len, cap}
	}

	// 动态增长
	// 当长度小于1024以2倍形式增长
	// 长度大于1024以1.25倍形式增长
	newcap := old.cap
	if newcap+newcap < cap {
		newcap = cap
	} else {
		for {
			if old.len < 1024 {
				newcap += newcap
			} else {
				newcap += newcap / 4
			}
			if newcap >= cap {
				break
			}
		}
	}

	if uintptr(newcap) >= _MaxMem/uintptr(et.size) {
		panic(errorString("growslice: cap out of range"))
	}
	// 计算原slice占用的内存大小
	lenmem := uintptr(old.len) * uintptr(et.size)
	// 需要分配的容量占用的内存大小, 并对齐
	capmem := roundupsize(uintptr(newcap) * uintptr(et.size))
	// 对齐子后的容量大小
	newcap = int(capmem / uintptr(et.size))
	var p unsafe.Pointer
	// 元素的类型不是指针，直接分配内存数据， 并清零cap-len的内存区
	if et.kind&kindNoPointers != 0 {
		p = rawmem(capmem)
		memmove(p, old.array, lenmem)
		memclr(add(p, lenmem), capmem-lenmem)
	} else {
		p = newarray(et, uintptr(newcap))
		if !writeBarrierEnabled {
			memmove(p, old.array, lenmem)
		} else {
			for i := uintptr(0); i < lenmem; i += et.size {
				typedmemmove(et, add(p, i), add(old.array, i))
			}
		}
	}

	return slice{p, old.len, newcap}
}

// slice的深拷贝， 将fm中的内容拷贝到to中， width代表拷贝元素的字节宽度
func slicecopy(to, fm slice, width uintptr) int {
	if fm.len == 0 || to.len == 0 {
		return 0
	}

	// 取fm和to中个数较小的长度
	n := fm.len
	if to.len < n {
		n = to.len
	}

	if width == 0 {
		return n
	}

	// 需要复制的内存大小
	size := uintptr(n) * width
	if size == 1 {
		*(*byte)(to.array) = *(*byte)(fm.array)
	} else {
		// 复制内容
		memmove(to.array, fm.array, size)
	}

	// 返回复制的元素个数
	return int(n)
}

// 字符串的深拷贝
func slicestringcopy(to []byte, fm string) int {
	if len(fm) == 0 || len(to) == 0 {
		return 0
	}

	n := len(fm)
	if len(to) < n {
		n = len(to)
	}

	memmove(unsafe.Pointer(&to[0]), unsafe.Pointer((*stringStruct)(unsafe.Pointer(&fm)).str), uintptr(n))
	return n
}

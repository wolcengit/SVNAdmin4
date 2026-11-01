// Copyright (C) 2026 wolcen@msn.com
// SPDX-License-Identifier: GPL-3.0-or-later
//
// SVNAdmin4 is free software: you can redistribute it and/or modify it under
// the terms of the GNU General Public License as published by the Free Software
// Foundation, either version 3 of the License, or (at your option) any later
// version. See the LICENSE file in the project root for the full license text.

package svn

import (
	"crypto/md5"
	"crypto/rand"
	"encoding/base64"
	"strings"
)

const (
	apr1Magic  = "$apr1$"
	apr1SaltLn = 8
	itoa64     = "./0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"
)

// Apr1Hash 生成 Apache htpasswd 使用的 apr1(MD5) 散列
func Apr1Hash(plain string) (string, error) {
	salt := make([]byte, apr1SaltLn)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}
	var sb strings.Builder
	for _, b := range salt {
		sb.WriteByte(itoa64[int(b)%64])
	}
	return apr1(plain, sb.String()), nil
}

// Apr1Verify 校验明文是否匹配 apr1 散列
func Apr1Verify(plain, hash string) bool {
	if !strings.HasPrefix(hash, apr1Magic) {
		return false
	}
	rest := hash[len(apr1Magic):]
	i := strings.Index(rest, "$")
	if i < 0 {
		return false
	}
	return apr1(plain, rest[:i]) == hash
}

func apr1(plain, salt string) string {
	if len(salt) > 8 {
		salt = salt[:8]
	}

	// final = md5(password + magic + salt)
	h := md5.New()
	h.Write([]byte(plain))
	h.Write([]byte(apr1Magic))
	h.Write([]byte(salt))

	// ctx1 = md5(password + salt + password)
	c1 := md5.New()
	c1.Write([]byte(plain))
	c1.Write([]byte(salt))
	c1.Write([]byte(plain))
	sum1 := c1.Sum(nil)

	for i := len(plain); i > 0; i -= 16 {
		if i > 16 {
			h.Write(sum1[:16])
		} else {
			h.Write(sum1[:i])
		}
	}
	for i := len(plain); i != 0; i >>= 1 {
		if i&1 != 0 {
			h.Write([]byte{0})
		} else {
			h.Write([]byte{plain[0]})
		}
	}
	final := h.Sum(nil)

	// 1000 轮混淆迭代
	for i := 0; i < 1000; i++ {
		n := md5.New()
		if i&1 != 0 {
			n.Write([]byte(plain))
		} else {
			n.Write(final)
		}
		if i%3 != 0 {
			n.Write([]byte(salt))
		}
		if i%7 != 0 {
			n.Write([]byte(plain))
		}
		if i&1 != 0 {
			n.Write(final)
		} else {
			n.Write([]byte(plain))
		}
		final = n.Sum(nil)
	}

	return apr1Encode(final, salt)
}

// apr1Encode 按 Apache 的重排规则把 16 字节摘要编码为 22 字符
func apr1Encode(final []byte, salt string) string {
	var out strings.Builder
	out.Grow(len(apr1Magic) + len(salt) + 23)
	out.WriteString(apr1Magic)
	out.WriteString(salt)
	out.WriteString("$")

	groups := [][3]int{
		{0, 6, 12},
		{1, 7, 13},
		{2, 8, 14},
		{3, 9, 15},
		{4, 10, 5},
	}
	for _, g := range groups {
		val := int(final[g[0]])<<16 | int(final[g[1]])<<8 | int(final[g[2]])
		for k := 0; k < 4; k++ {
			out.WriteByte(itoa64[val&0x3f])
			val >>= 6
		}
	}
	val := int(final[11])
	for k := 0; k < 2; k++ {
		out.WriteByte(itoa64[val&0x3f])
		val >>= 6
	}
	return out.String()
}

// BasicHash 生成 HTTP BASIC 认证的 base64 值（仅用于展示）
func BasicHash(user, plain string) string {
	return base64.StdEncoding.EncodeToString([]byte(user + ":" + plain))
}

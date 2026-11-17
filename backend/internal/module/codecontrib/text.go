package codecontrib

import "unicode/utf8"

// TruncateUTF8 limits s to max bytes without splitting a multi-byte
// character: when the cut lands inside one, the incomplete trailing bytes
// are dropped. 按字节截断 provider 文本（提交标题、diff 补丁、错误信息）时，
// 切点落在多字节字符中间会留下不完整的尾序列，Postgres 会以
// `invalid byte sequence for encoding "UTF8"` 拒绝整批写入，进而让整个
// 仓库的同步在每轮重试里无限失败（SQLSTATE 22021）。
func TruncateUTF8(s string, max int) string {
	if max <= 0 || len(s) <= max {
		return s
	}
	cut := s[:max]
	for len(cut) > 0 {
		r, size := utf8.DecodeLastRuneInString(cut)
		if r != utf8.RuneError || size > 1 {
			return cut
		}
		cut = cut[:len(cut)-size]
	}
	return cut
}

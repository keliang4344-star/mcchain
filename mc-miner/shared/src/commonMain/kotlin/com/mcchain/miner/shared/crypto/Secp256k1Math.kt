package com.mcchain.miner.shared.crypto

/**
 * 256 位无符号整数与 secp256k1 域/曲线运算（纯 Kotlin，commonMain 可用）。
 *
 * 存在意义：iOS 目标没有 BouncyCastle，交易签名必须能在 commonMain 完成；
 * 本文件与 jvmMain 的 BouncyCastle 实现在 jvmTest 中逐字节对照
 * （RFC6979 确定性签名 ⇒ r||s 必须一致），并对照 mcchaind 生成的 golden 向量。
 *
 * 表示：8 个 32 位 limb 的小端序数组（l[0] 为最低位）。
 * 性能定位：移动端每天几十笔签名，double-and-add + 特殊形约减足够；
 * 非恒定时间实现（私钥不出设备，时序侧信道风险可接受，见方案威胁模型）。
 */
internal object Secp256k1Math {

    class Mod(val limbs: IntArray, val c: IntArray) {
        /** 2^256 - modulus 的小端 limb 表示（fold 约减用）。 */
    }

    /** secp256k1 基域 p = 2^256 - 0x1000003D1 */
    val P = Mod(
        limbs = intArrayOf(
            0xFFFFFC2F.toInt(), 0xFFFFFFFE.toInt(), -1, -1, -1, -1, -1, 0xFFFFFFFF.toInt(),
        ),
        c = intArrayOf(0x000003D1, 0x00000001),
    )

    /** 曲线阶 n */
    val N = Mod(
        limbs = intArrayOf(
            0xD0364141.toInt(), 0xBFD25E8C.toInt(), 0xAF48A03B.toInt(), 0xBAAEDCE6.toInt(),
            0xFFFFFFFE.toInt(), -1, -1, 0xFFFFFFFF.toInt(),
        ),
        c = intArrayOf(0x2FC9BEBF, 0x402DA173, 0x50B75FC4, 0x45512319, 0x00000001),
    )

    // ------------------------------------------------------------------
    // 基础原语（输入输出均为小端 IntArray，每 limb 按 UInt 语义解释）
    // ------------------------------------------------------------------

    fun fromBytesBE(bytes: ByteArray): IntArray {
        require(bytes.size == 32) { "expected 32 bytes, got ${bytes.size}" }
        val out = IntArray(8)
        for (i in 0 until 8) {
            val o = (7 - i) * 4
            out[i] = ((bytes[o].toInt() and 0xFF) shl 24) or
                ((bytes[o + 1].toInt() and 0xFF) shl 16) or
                ((bytes[o + 2].toInt() and 0xFF) shl 8) or
                (bytes[o + 3].toInt() and 0xFF)
        }
        return out
    }

    fun toBytesBE(limbs: IntArray): ByteArray {
        val out = ByteArray(32)
        for (i in 0 until 8) {
            val o = (7 - i) * 4
            val v = limbs[i]
            out[o] = (v ushr 24).toByte()
            out[o + 1] = (v ushr 16).toByte()
            out[o + 2] = (v ushr 8).toByte()
            out[o + 3] = v.toByte()
        }
        return out
    }

    /** a >= b ? （无符号） */
    fun gte(a: IntArray, b: IntArray): Boolean {
        for (i in 7 downTo 0) {
            val x = a[i].toUInt()
            val y = b[i].toUInt()
            if (x != y) return x > y
        }
        return true
    }

    fun isZero(a: IntArray): Boolean = a.all { it == 0 }

    /** a = a - b（要求 a >= b） */
    fun subAssign(a: IntArray, b: IntArray) {
        var borrow = 0L
        for (i in 0 until 8) {
            val d = (a[i].toLong() and 0xFFFFFFFFL) - (b[i].toLong() and 0xFFFFFFFFL) - borrow
            a[i] = d.toInt()
            borrow = if (d < 0) 1 else 0
        }
        require(borrow == 0L) { "subAssign underflow" }
    }

    /** (a + b) mod m */
    fun addMod(a: IntArray, b: IntArray, m: Mod): IntArray {
        val r = IntArray(8)
        var carry = 0L
        for (i in 0 until 8) {
            val s = (a[i].toLong() and 0xFFFFFFFFL) + (b[i].toLong() and 0xFFFFFFFFL) + carry
            r[i] = s.toInt()
            carry = s ushr 32
        }
        if (carry != 0L) {
            // a+b ≥ 2^256：r = a+b-2^256，需补回 2^256 - m = c，结果必 < m
            var c = 0L
            for (i in 0 until m.c.size) {
                val s = (r[i].toLong() and 0xFFFFFFFFL) + (m.c[i].toLong() and 0xFFFFFFFFL) + c
                r[i] = s.toInt()
                c = s ushr 32
            }
            var i = m.c.size
            while (c != 0L && i < 8) {
                val s = (r[i].toLong() and 0xFFFFFFFFL) + c
                r[i] = s.toInt()
                c = s ushr 32
                i++
            }
            return r
        }
        if (gte(r, m.limbs)) subAssign(r, m.limbs)
        return r
    }

    /** (a - b) mod m */
    fun subMod(a: IntArray, b: IntArray, m: Mod): IntArray {
        val r = IntArray(8)
        var borrow = 0L
        for (i in 0 until 8) {
            val d = (a[i].toLong() and 0xFFFFFFFFL) - (b[i].toLong() and 0xFFFFFFFFL) - borrow
            r[i] = d.toInt()
            borrow = if (d < 0) 1 else 0
        }
        if (borrow != 0L) {
            var c = 0L
            for (i in 0 until 8) {
                val s = (r[i].toLong() and 0xFFFFFFFFL) + (m.limbs[i].toLong() and 0xFFFFFFFFL) + c
                r[i] = s.toInt()
                c = s ushr 32
            }
        }
        return r
    }

    /** a * b → 16 limb（小端，每 limb < 2^32，累积在 ULong 不溢出：8×(2^32-1)^2 + carry < 2^64） */
    fun mulWide(a: IntArray, b: IntArray): ULongArray {
        val r = ULongArray(16)
        for (i in 0 until 8) {
            var carry = 0UL
            val ai = a[i].toULong() and 0xFFFFFFFFUL
            for (j in 0 until 8) {
                val cur = r[i + j] + ai * (b[j].toULong() and 0xFFFFFFFFUL) + carry
                r[i + j] = cur and 0xFFFFFFFFUL
                carry = cur shr 32
            }
            r[i + 8] += carry
        }
        return r
    }

    /**
     * 16 limb → 8 limb：折叠 v_hi * 2^256 ≡ v_hi * c (mod m)，
     * 循环直到无高位，最后条件减法。c 短（p 用 2 limb、n 用 5 limb），收敛快。
     */
    fun reduce(v: ULongArray, m: Mod): IntArray {
        val r = ULongArray(16)
        v.copyInto(r)
        while (true) {
            var any = false
            for (i in 8 until 16) if (r[i] != 0UL) { any = true; break }
            if (!any) break
            val hi = ULongArray(8) { r[it + 8] }
            for (i in 8 until 16) r[i] = 0UL
            // r += hi * c
            var carry = 0UL
            for (j in m.c.indices) {
                val cj = m.c[j].toULong() and 0xFFFFFFFFUL
                var k = 0
                while (k < 8) {
                    if (hi[k] == 0UL && carry == 0UL) break
                    val cur = r[j + k] + hi[k] * cj + carry
                    r[j + k] = cur and 0xFFFFFFFFUL
                    carry = cur shr 32
                    k++
                }
                var pos = j + 8
                while (carry != 0UL && pos < 16) {
                    val cur = r[pos] + carry
                    r[pos] = cur and 0xFFFFFFFFUL
                    carry = cur shr 32
                    pos++
                }
            }
        }
        val out = IntArray(8) { r[it].toInt() }
        while (gte(out, m.limbs)) subAssign(out, m.limbs)
        return out
    }

    /** (a * b) mod m */
    fun mulMod(a: IntArray, b: IntArray, m: Mod): IntArray = reduce(mulWide(a, b), m)

    /** a^e mod m（LSB-first square-and-multiply：base 逐位平方，命中位乘入 result） */
    fun powMod(a: IntArray, e: IntArray, m: Mod): IntArray {
        var result = IntArray(8)
        result[0] = 1
        var base = a.copyOf()
        for (bit in 0 until 256) {
            if ((e[bit / 32].toUInt() shr (bit % 32)) and 1u == 1u) {
                result = mulMod(result, base, m)
            }
            base = mulMod(base, base, m)
        }
        return result
    }

    /** a^(m-2) mod m：费马小定理逆元（m 为素数）。0 的逆元返回 0。 */
    fun invMod(a: IntArray, m: Mod): IntArray {
        if (isZero(a)) return IntArray(8)
        val e = IntArray(8)
        // m - 2
        var borrow = 2L
        for (i in 0 until 8) {
            val d = (m.limbs[i].toLong() and 0xFFFFFFFFL) - borrow
            e[i] = d.toInt()
            borrow = if (d < 0) 1 else 0
        }
        return powMod(a, e, m)
    }

    // ------------------------------------------------------------------
    // 仿射点与 Jacobian 标量乘（a=0 曲线公式）
    // ------------------------------------------------------------------

    class Point(val x: IntArray, val y: IntArray, val z: IntArray)

    private val ZERO = IntArray(8)

    /** Jacobian 倍点（secp256k1: a = 0） */
    fun doublePoint(p: Point): Point {
        if (isZero(p.z) || isZero(p.y)) return Point(ZERO, ZERO, ZERO)
        val a = mulMod(p.x, p.x, P)
        val b = mulMod(p.y, p.y, P)
        val c = mulMod(b, b, P)
        val xb = addMod(p.x, b, P)
        val d = subMod(subMod(mulMod(xb, xb, P), a, P), c, P)
        val dd = addMod(d, d, P)
        val e = addMod(addMod(a, a, P), a, P)
        val f = mulMod(e, e, P)
        val x3 = subMod(subMod(f, dd, P), dd, P)
        val eightC = mulMod(c, intArrayOf(8, 0, 0, 0, 0, 0, 0, 0), P)
        val y3 = subMod(mulMod(e, subMod(dd, x3, P), P), eightC, P)
        val z3 = mulMod(addMod(p.y, p.y, P), p.z, P)
        return Point(x3, y3, z3)
    }

    /** Jacobian 通加（p != q，非无穷） */
    fun addPoints(p: Point, q: Point): Point {
        if (isZero(p.z)) return q
        if (isZero(q.z)) return p
        val z1z1 = mulMod(p.z, p.z, P)
        val z2z2 = mulMod(q.z, q.z, P)
        val u1 = mulMod(p.x, z2z2, P)
        val u2 = mulMod(q.x, z1z1, P)
        val s1 = mulMod(mulMod(p.y, q.z, P), z2z2, P)
        val s2 = mulMod(mulMod(q.y, p.z, P), z1z1, P)
        if (u1 contentEquals u2) {
            return if (s1 contentEquals s2) doublePoint(p) else Point(ZERO, ZERO, ZERO)
        }
        val h = subMod(u2, u1, P)
        val i = mulMod(addMod(h, h, P), addMod(h, h, P), P)
        val j = mulMod(h, i, P)
        val r = subMod(s2, s1, P)
        val rr = addMod(r, r, P)
        val v = mulMod(u1, i, P)
        val x3 = subMod(subMod(mulMod(rr, rr, P), j, P), addMod(v, v, P), P)
        val y3 = subMod(mulMod(rr, subMod(v, x3, P), P), mulMod(addMod(s1, s1, P), j, P), P)
        // Z3 = ((Z1+Z2)^2 - Z1Z1 - Z2Z2) * H  （即 2*Z1*Z2*H）
        val zsum = addMod(p.z, q.z, P)
        val z3 = mulMod(subMod(subMod(mulMod(zsum, zsum, P), z1z1, P), z2z2, P), h, P)
        return Point(x3, y3, z3)
    }

    /** k * G（G 为 secp256k1 生成元，仿射坐标） */
    fun mulG(k: IntArray): Point = mulPoint(GEN_X, GEN_Y, k)

    /** k * (GEN_X, GEN_Y)，MSB→LSB double-and-add */
    fun mulPoint(gx: IntArray, gy: IntArray, k: IntArray): Point {
        var acc: Point? = null
        var cur = Point(gx, gy, one())
        for (bit in 0 until 256) {
            if ((k[bit / 32].toUInt() shr (bit % 32)) and 1u == 1u) {
                acc = if (acc == null) cur else addPoints(acc, cur)
            }
            cur = doublePoint(cur)
        }
        return acc ?: Point(ZERO, ZERO, ZERO)
    }

    /** Jacobian → 仿射 */
    fun toAffine(p: Point): Pair<IntArray, IntArray> {
        val zi = invMod(p.z, P)
        val zi2 = mulMod(zi, zi, P)
        val x = mulMod(p.x, zi2, P)
        val y = mulMod(p.y, mulMod(zi2, zi, P), P)
        return x to y
    }

    /** 33 字节压缩公钥：0x02/0x03 || x（大端） */
    fun compressedPubKey(x: IntArray, y: IntArray): ByteArray {
        val prefix = if ((y[0].toUInt() and 1u) == 1u) 0x03.toByte() else 0x02.toByte()
        return byteArrayOf(prefix) + toBytesBE(x)
    }

    private fun one(): IntArray = IntArray(8).also { it[0] = 1 }

    // G 生成元（secp256k1 标准常量，小端 32-bit limb）
    // Gx = 79BE667E F9DCBBAC 55A06295 CE870B07 029BFCDB 2DCE28D9 59F2815B 16F81798
    val GEN_X = intArrayOf(
        0x16F81798, 0x59F2815B, 0x2DCE28D9, 0x029BFCDB,
        0xCE870B07.toInt(), 0x55A06295, 0xF9DCBBAC.toInt(), 0x79BE667E,
    )
    // Gy = 483ADA77 26A3C465 5DA4FBFC 0E1108A8 FD17B448 A6855419 9C47D08F FB10D4B8
    val GEN_Y = intArrayOf(
        0xFB10D4B8.toInt(), 0x9C47D08F.toInt(), 0xA6855419.toInt(), 0xFD17B448.toInt(),
        0x0E1108A8, 0x5DA4FBFC, 0x26A3C465, 0x483ADA77,
    )

    /** n/2（low-S 判定阈值）= 7FFFFFFF FFFFFFFF FFFFFFFF 7FFFFFFF 5D576E73 57A4501D DFE92F46 681B20A0 */
    val N_HALF = intArrayOf(
        0x681B20A0, 0xDFE92F46.toInt(), 0x57A4501D, 0x5D576E73,
        0x7FFFFFFF, -1, -1, 0x7FFFFFFF,
    )
}

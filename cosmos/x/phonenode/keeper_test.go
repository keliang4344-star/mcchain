package phonenode

import (
	"testing"
)

// 构造一棵 4 叶 Merkle 树，返回 root 与每个叶子的正确 proof。
//
// 树结构：
//
//	root = H(N0, N1)
//	N0 = H(L0, L1),  N1 = H(L2, L3)
//
// 证明方向由 leaf 的 index 二进制位决定：
//   - bit i = 0 → 本节点在左，兄弟在右：H(node, sibling)
//   - bit i = 1 → 本节点在右，兄弟在左：H(sibling, node)
func buildFourLeaves() (root []byte, leaves [][]byte, proofs [][][]byte) {
	leaves = [][]byte{
		LeafHash([]byte("balance:A:1000")),
		LeafHash([]byte("balance:B:30")),
		LeafHash([]byte("contrib:MCd1:425")),
		LeafHash([]byte("contrib:MCd2:210")),
	}
	l0 := HashPair(leaves[0], leaves[1]) // N0
	l1 := HashPair(leaves[2], leaves[3]) // N1
	root = HashPair(l0, l1)

	// 每个 leaf 的 proof = [本层兄弟, 上层兄弟]
	proofs = [][][]byte{
		{leaves[1], l1}, // L0 (idx0): 右兄弟 L1，再上层右兄弟 N1
		{leaves[0], l1}, // L1 (idx1): 左兄弟 L0，再上层右兄弟 N1
		{leaves[3], l0}, // L2 (idx2): 右兄弟 L3，再上层左兄弟 N0
		{leaves[2], l0}, // L3 (idx3): 左兄弟 L2，再上层左兄弟 N0
	}
	return
}

func TestMerkleProofValid(t *testing.T) {
	root, leaves, proofs := buildFourLeaves()
	for i := range leaves {
		if !VerifyMerkleProof(root, leaves[i], proofs[i], i) {
			t.Fatalf("leaf %d proof should verify", i)
		}
	}
}

func TestMerkleProofTampered(t *testing.T) {
	root, leaves, proofs := buildFourLeaves()
	// 篡改 leaf0 内容，证明应失败
	bad := LeafHash([]byte("balance:A:999999"))
	if VerifyMerkleProof(root, bad, proofs[0], 0) {
		t.Fatal("tampered leaf must fail verification")
	}
	// 用错 index
	if VerifyMerkleProof(root, leaves[0], proofs[0], 1) {
		t.Fatal("wrong index must fail")
	}
}

func TestLightNodeLifecycle(t *testing.T) {
	k := NewKeeper()
	n, err := k.RegisterNode("MCphone1", "iPhone15", "iOS17", RoleLight)
	if err != nil {
		t.Fatalf("register: %v", err)
	}
	if n.Role != RoleLight {
		t.Fatal("role mismatch")
	}
	if _, err := k.RegisterNode("MCphone1", "x", "y", RoleLight); err != ErrNodeExists {
		t.Fatalf("expected ErrNodeExists, got %v", err)
	}
	root, leaves, proofs := buildFourLeaves()
	ok, err := k.SubmitStateProof("MCphone1", root, leaves[0], 0, proofs[0])
	if !ok || err != nil {
		t.Fatalf("proof should pass: ok=%v err=%v", ok, err)
	}
	// 错误证明（篡改 leaf）
	bad := LeafHash([]byte("x"))
	if _, err := k.SubmitStateProof("MCphone1", root, bad, 0, proofs[0]); err != ErrBadProof {
		t.Fatalf("expected ErrBadProof, got %v", err)
	}
	// 未知节点
	if _, err := k.SubmitStateProof("MCghost", root, leaves[0], 0, proofs[0]); err != ErrNodeNotFound {
		t.Fatalf("expected ErrNodeNotFound, got %v", err)
	}
	st, _ := k.GetNode("MCphone1")
	if st.ProofsOK != 1 || st.ProofsBad != 1 {
		t.Fatalf("proof stats wrong: ok=%d bad=%d", st.ProofsOK, st.ProofsBad)
	}
	if err := k.MarkPruned("MCphone1"); err != nil {
		t.Fatalf("prune: %v", err)
	}
	if !st.Pruned {
		t.Fatal("should be pruned")
	}
	if k.CountNodes() != 1 {
		t.Fatal("node count wrong")
	}
}

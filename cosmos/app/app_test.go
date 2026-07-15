package app

import (
	"testing"

	"mcchain-cosmos/x/phonenode"
)

// 构造 4 叶 Merkle 树用于轻节点证明演示。
func sampleMerkle() (root []byte, leaves [][]byte, proofs [][][]byte) {
	leaves = [][]byte{
		phonenode.LeafHash([]byte("balance:MCdevA:1000")),
		phonenode.LeafHash([]byte("balance:MCdevB:30")),
		phonenode.LeafHash([]byte("contrib:MCdevA:425")),
		phonenode.LeafHash([]byte("contrib:MCdevB:210")),
	}
	l0 := phonenode.HashPair(leaves[0], leaves[1])
	l1 := phonenode.HashPair(leaves[2], leaves[3])
	root = phonenode.HashPair(l0, l1)
	proofs = [][][]byte{
		{leaves[1], l1},
		{leaves[0], l1},
		{leaves[3], l0},
		{leaves[2], l0},
	}
	return
}

func TestAppDePINBlockLoop(t *testing.T) {
	a := New()

	// 1) 注册贡献设备
	if err := a.RegisterDevice("MCdevA", "Pixel8", "Android14"); err != nil {
		t.Fatalf("register A: %v", err)
	}
	if err := a.RegisterDevice("MCdevB", "iPhone15", "iOS17"); err != nil {
		t.Fatalf("register B: %v", err)
	}

	// 2) 提交贡献到交易池（尚未上链）
	a.SubmitContribution("MCdevA", "t1", "inference", 85)   // 425
	a.SubmitContribution("MCdevA", "t2", "data_label", 80)  // 240
	a.SubmitContribution("MCdevB", "t3", "bandwidth", 50)   // 50
	a.SubmitContribution("MCdevA", "t4", "data_label", 20)  // 低于阈值 → 0
	if a.PendingCount() != 4 {
		t.Fatalf("pending should be 4, got %d", a.PendingCount())
	}

	// 3) 出块：打包所有贡献
	n, err := a.Commit()
	if err != nil {
		t.Fatalf("commit: %v", err)
	}
	if n != 4 {
		t.Fatalf("processed should be 4, got %d", n)
	}
	if a.Height != 1 {
		t.Fatalf("height should be 1, got %d", a.Height)
	}
	// mint = 425 + 240 + 50 + 0 = 715
	if a.Minted != 715 {
		t.Fatalf("minted should be 715, got %d", a.Minted)
	}
	// 设备 A 累计 425+240+0 = 665
	ra, _ := a.DeviceReward("MCdevA")
	if ra != 665 {
		t.Fatalf("devA reward should be 665, got %d", ra)
	}
	rb, _ := a.DeviceReward("MCdevB")
	if rb != 50 {
		t.Fatalf("devB reward should be 50, got %d", rb)
	}
	if a.PendingCount() != 0 {
		t.Fatal("pending should be cleared after commit")
	}

	// 4) 第二个区块：再提交一条，验证链高递增
	a.SubmitContribution("MCdevB", "t5", "inference", 100) // 500 封顶
	a.Commit()
	if a.Height != 2 {
		t.Fatalf("height should be 2, got %d", a.Height)
	}
	if a.Minted != 1215 { // 715 + 500
		t.Fatalf("minted should be 1215, got %d", a.Minted)
	}
}

func TestAppLightNodeProof(t *testing.T) {
	a := New()
	if _, err := a.PhoneNode.RegisterNode("MCphoneX", "Pixel8", "Android14", phonenode.RoleLight); err != nil {
		t.Fatalf("register node: %v", err)
	}
	root, leaves, proofs := sampleMerkle()
	ok, err := a.SubmitLightProof("MCphoneX", root, leaves[0], 0, proofs[0])
	if !ok || err != nil {
		t.Fatalf("proof should pass: ok=%v err=%v", ok, err)
	}
	bad := phonenode.LeafHash([]byte("tampered"))
	if _, err := a.SubmitLightProof("MCphoneX", root, bad, 0, proofs[0]); err == nil {
		t.Fatal("tampered proof must fail")
	}
}

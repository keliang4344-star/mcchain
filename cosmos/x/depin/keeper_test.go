package depin

import "testing"

func TestRegisterAndRewardFlow(t *testing.T) {
	k := NewKeeper()

	dev, err := k.RegisterDevice("MCdevA", "Pixel8", "Android14")
	if err != nil {
		t.Fatalf("register: %v", err)
	}
	if !dev.Registered {
		t.Fatal("device should be registered")
	}
	// 重复注册应报错
	if _, err := k.RegisterDevice("MCdevA", "x", "y"); err != ErrDeviceExists {
		t.Fatalf("expected ErrDeviceExists, got %v", err)
	}

	// inference 85 → 85*5 = 425
	r, err := k.SubmitAndReward("t1", "MCdevA", TaskTypeInference, 85)
	if err != nil {
		t.Fatalf("reward t1: %v", err)
	}
	if r != 425 {
		t.Fatalf("expected 425, got %d", r)
	}

	// 低于阈值 → 0 奖励但记录
	r, err = k.SubmitAndReward("t2", "MCdevA", TaskTypeDataLabel, 20)
	if err != nil {
		t.Fatalf("reward t2: %v", err)
	}
	if r != 0 {
		t.Fatalf("expected 0 (below threshold), got %d", r)
	}

	// 封顶：inference 100 → 500
	r, err = k.SubmitAndReward("t3", "MCdevA", TaskTypeInference, 100)
	if err != nil {
		t.Fatalf("reward t3: %v", err)
	}
	if r != 500 {
		t.Fatalf("expected 500 capped, got %d", r)
	}

	// 非法类型
	if _, err := k.SubmitAndReward("t4", "MCdevA", "hack", 80); err != ErrUnsupportedType {
		t.Fatalf("expected ErrUnsupportedType, got %v", err)
	}
	// 越界分数
	if _, err := k.SubmitAndReward("t5", "MCdevA", TaskTypeBandwidth, 200); err != ErrInvalidScore {
		t.Fatalf("expected ErrInvalidScore, got %v", err)
	}
	// 重复 taskID
	if _, err := k.SubmitAndReward("t1", "MCdevA", TaskTypeBandwidth, 50); err != ErrTaskExists {
		t.Fatalf("expected ErrTaskExists, got %v", err)
	}
	// 未知设备
	if _, err := k.SubmitAndReward("t6", "MCghost", TaskTypeBandwidth, 50); err != ErrDeviceNotFound {
		t.Fatalf("expected ErrDeviceNotFound, got %v", err)
	}

	total, _ := k.DeviceReward("MCdevA")
	if total != 925 { // 425 + 0 + 500
		t.Fatalf("expected total 925, got %d", total)
	}
	if k.CountDevices() != 1 || k.CountContributions() != 3 {
		t.Fatalf("count mismatch dev=%d contrib=%d", k.CountDevices(), k.CountContributions())
	}
}

func TestDeterministicContributionOrder(t *testing.T) {
	k := NewKeeper()
	k.RegisterDevice("MCd", "x", "y")
	for i := 0; i < 5; i++ {
		id := string(rune('a' + i))
		if _, err := k.SubmitAndReward(id, "MCd", TaskTypeBandwidth, 50); err != nil {
			t.Fatal(err)
		}
	}
	list := k.AllContributions()
	if len(list) != 5 {
		t.Fatalf("expected 5, got %d", len(list))
	}
	for i, c := range list {
		if c.TaskID != string(rune('a'+i)) {
			t.Fatalf("order broken at %d: %s", i, c.TaskID)
		}
	}
}

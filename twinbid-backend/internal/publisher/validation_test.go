package publisher

import "testing"

func TestPayoutWalletChecksum(t *testing.T) {
	valid := "T9yD14Nj9j7xAB4dbGeiX9h8unkKHxuWwb"
	if !validWallet("usdt_trc20", valid) {
		t.Fatal("valid Base58Check rejected")
	}
	if validWallet("usdt_trc20", valid[:len(valid)-1]+"a") {
		t.Fatal("bad checksum accepted")
	}
	if validWallet("usdt_erc20", "0x123") {
		t.Fatal("short ERC address accepted")
	}
	if validWallet("bank", valid) {
		t.Fatal("unsupported payout method")
	}
}

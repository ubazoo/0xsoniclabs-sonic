package gossip

import (
	"math/big"
	"testing"
	"time"

	"github.com/0xsoniclabs/sonic/evmcore"
	"go.uber.org/mock/gomock"
)

func TestServiceFeed_ExampleTest(t *testing.T) {
	ctrl := gomock.NewController(t)
	store := NewMockArchiveBlockHeightSource(ctrl)

	store.EXPECT().GetArchiveBlockHeight().Return(uint64(12), false, nil).AnyTimes()

	feed := ServiceFeed{}
	feed.Start(store)

	consumer := make(chan evmcore.ChainHeadNotify, 1)
	feed.SubscribeNewBlock(consumer)

	// There should be no signal delivered until there is a notification.
	select {
	case <-consumer:
		t.Fatal("expected no notification to be sent")
	case <-time.After(100 * time.Millisecond):
		// all good
	}

	feed.notifyAboutNewBlock(&evmcore.EvmBlock{
		EvmHeader: evmcore.EvmHeader{
			Number: big.NewInt(12),
		},
	}, nil)

	// The notification should be delivered.
	select {
	case notification := <-consumer:
		if notification.Block.Number.Cmp(big.NewInt(12)) != 0 {
			t.Fatalf("expected block number 12, got %d", notification.Block.Number)
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("expected notification to be sent")
	}

	feed.Stop()
}

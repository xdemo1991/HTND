package testing

import (
	"testing"
	"time"

	"github.com/Hoosat-Oy/htnd/app/protocol/flows/v5/addressexchange"

	"github.com/Hoosat-Oy/htnd/app/appmessage"
	peerpkg "github.com/Hoosat-Oy/htnd/app/protocol/peer"
	"github.com/Hoosat-Oy/htnd/domain/consensus"
	"github.com/Hoosat-Oy/htnd/domain/consensus/utils/testutils"
	"github.com/Hoosat-Oy/htnd/infrastructure/network/addressmanager"
	"github.com/Hoosat-Oy/htnd/infrastructure/network/netadapter/router"
)

type fakeReceiveAddressesContext struct{}

func (f fakeReceiveAddressesContext) AddressManager() *addressmanager.AddressManager {
	return nil
}

func TestReceiveAddressesErrors(t *testing.T) {
	testutils.ForAllNets(t, true, func(t *testing.T, consensusConfig *consensus.Config) {
		incomingRoute := router.NewRoute("incoming")
		outgoingRoute := router.NewRoute("outgoing")
		peer := peerpkg.New(nil)
		errChan := make(chan error)
		go func() {
			errChan <- addressexchange.ReceiveAddresses(fakeReceiveAddressesContext{}, incomingRoute, outgoingRoute, peer)
		}()

		_, err := outgoingRoute.DequeueWithTimeout(time.Second)
		if err != nil {
			t.Fatalf("DequeueWithTimeout: %+v", err)
		}

		// Sending addressmanager.GetAddressesMax+1 addresses should trigger a ban
		err = incomingRoute.Enqueue(appmessage.NewMsgAddresses(make([]*appmessage.NetAddress,
			addressmanager.GetAddressesMax+1)))
		if err != nil {
			t.Fatalf("Enqueue: %+v", err)
		}

		select {
		case err := <-errChan:
			checkFlowError(t, err, true, true, "address count exceeded")
		case <-time.After(time.Second):
			t.Fatalf("timed out after %s", time.Second)
		}
	})
}

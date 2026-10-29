package factualapi

import (
	"reflect"
	"testing"
)

func TestSKUInboundNeverPublishesSyntheticFutureStock(t *testing.T) {
	typ := reflect.TypeOf(SKUInbound{})
	for _, name := range []string{"FutureStock", "ExpectedStockAt", "Stock"} {
		if _, ok := typ.FieldByName(name); ok {
			t.Fatalf("SKUInbound must not expose %s", name)
		}
	}
	if _, ok := typ.FieldByName("DeclaredInboundUnits"); !ok {
		t.Fatal("declared inbound units missing")
	}
}

func TestSupplyLocationsKeepDropoffAndStorageRolesSeparate(t *testing.T) {
	order := reflect.TypeOf(SupplyOrder{})
	supply := reflect.TypeOf(InboundSupply{})
	if _, ok := order.FieldByName("DropoffLocationID"); !ok {
		t.Fatal("dropoff location missing")
	}
	if _, ok := supply.FieldByName("StorageLocationID"); !ok {
		t.Fatal("storage location missing")
	}
}

package ipam

import (
	"net"
	"reflect"
	"runtime"
	"testing"
)

func TestNewMasSource(t *testing.T) {
	options := make(map[string]interface{})
	mas, _ := newMasSource(options)

	if runtime.GOOS == "windows" {
		if mas.filePath != defaultWindowsFilePath {
			t.Errorf("default file path set incorrectly")
		}
	} else {
		if mas.filePath != defaultLinuxFilePath {
			t.Errorf("default file path set incorrectly")
		}
	}
	if mas.name != "MAS" {
		t.Errorf("mas source Name incorrect")
	}
}

func TestGetSDNInterfaces(t *testing.T) {
	const validFileName = "testfiles/masInterfaceConfig.json"
	const invalidFileName = "mas_test.go"
	const nonexistentFileName = "bad"

	interfaces, err := getSDNInterfaces(validFileName)
	if err != nil {
		t.Fatalf("failed to get sdn Interfaces from file: %v", err)
	}

	correctInterfaces := []Interface{
		{
			Name:      "eth0",
			IsPrimary: false,
			IPSubnets: []IPSubent{
				{
					Prefix: "10.240.0.0/12",
					IPAddresses: []IPAddress{
						{Address: "10.240.0.4", IsPrimary: true},
						{Address: "10.240.0.5", IsPrimary: false},
					},
				},
			},
		},
		{
			MacAddress: "000D3A6E1825",
			IsPrimary:  true,
			IPSubnets: []IPSubent{
				{
					Prefix: "1.0.0.0/12",
					IPAddresses: []IPAddress{
						{Address: "1.0.0.4", IsPrimary: true},
						{Address: "1.0.0.5", IsPrimary: false},
						{Address: "1.0.0.6", IsPrimary: false},
						{Address: "1.0.0.7", IsPrimary: false},
					},
				},
			},
		},
	}

	if !reflect.DeepEqual(interfaces, correctInterfaces) {
		t.Fatalf("Interface list did not match expected list. expected: %v, actual: %v", interfaces, correctInterfaces)
	}

	interfaces, err = getSDNInterfaces(invalidFileName)
	if interfaces != nil || err == nil {
		t.Fatal("didn't throw error on invalid file")
	}

	interfaces, err = getSDNInterfaces(nonexistentFileName)
	if interfaces != nil || err == nil {
		t.Fatal("didn't throw error on nonexistent file")
	}
}

func TestPopulateAddressSpace(t *testing.T) {

	hardwareAddress, _ := net.ParseMAC("00:0d:3a:6e:18:25")
	localInterfaces := []net.Interface{{HardwareAddr: hardwareAddress, Name: "eth0"}}

	// Interfaces should match based on Mac Address

	local := newLocalAddressSpace()

	sdnInterfaces := []Interface{
		{
			MacAddress: "000D3A6E1825",
			IsPrimary:  true,
			IPSubnets: []IPSubent{
				{
					Prefix: "1.0.0.0/12",
					IPAddresses: []IPAddress{
						{Address: "1.1.1.5", IsPrimary: true},
						{Address: "1.1.1.6", IsPrimary: false},
						{Address: "1.1.1.6", IsPrimary: false},
						{Address: "invalid", IsPrimary: false},
					},
				},
				{
					Prefix: "1.0.0.0/12",
				},
				{
					Prefix: "invalid",
				},
			},
		},
	}

	err := populateAddressSpace(local, sdnInterfaces, localInterfaces)
	if err != nil {
		t.Fatalf("Error populating address space: %v", err)
	}

	if len(local.Pools) != 1 {
		t.Fatalf("Pool list has incorrect length. expected: %d, actual: %d", 1, len(local.Pools))
	}

	pool, ok := local.Pools["1.0.0.0/12"]
	if !ok {
		t.Fatal("Address pool 1.0.0.0/12 missing")
	}

	if pool.IfName != localInterfaces[0].Name {
		t.Fatalf("Incorrect interface name. expected: %s, actual %s", localInterfaces[0].Name, pool.IfName)
	}

	if pool.Priority != 0 {
		t.Fatalf("Incorrect interface priority. expected: %d, actual %d", 0, pool.Priority)
	}

	if len(pool.Addresses) != 1 {
		t.Fatalf("Address list has incorrect length. expected: %d, actual: %d", 1, len(pool.Addresses))
	}

	_, ok = pool.Addresses["1.1.1.6"]
	if !ok {
		t.Fatal("Address 1.1.1.6 missing")
	}

	// Interfaces should match based on Name when no Mac Address Present

	local = newLocalAddressSpace()

	sdnInterfaces = []Interface{
		{
			Name:      localInterfaces[0].Name,
			IsPrimary: false,
			IPSubnets: []IPSubent{{Prefix: "2.0.0.0/12"}},
		},
	}

	err = populateAddressSpace(local, sdnInterfaces, localInterfaces)
	if err != nil {
		t.Fatalf("Error populating address space: %v", err)
	}

	if len(local.Pools) != 1 {
		t.Fatalf("Pool list has incorrect length. expected: %d, actual: %d", 1, len(local.Pools))
	}

	pool, ok = local.Pools["2.0.0.0/12"]
	if !ok {
		t.Fatal("Address pool 2.0.0.0/12 missing")
	}

	if pool.IfName != localInterfaces[0].Name {
		t.Fatalf("Incorrect interface name. expected: %s, actual %s", localInterfaces[0].Name, pool.IfName)
	}

	if pool.Priority != 1 {
		t.Fatalf("Incorrect interface priority. expected: %d, actual %d", 1, pool.Priority)
	}

	// Interfaces should not match based on Name when Mac Address is Present

	local = newLocalAddressSpace()

	sdnInterfaces = []Interface{
		{
			MacAddress: "invalid",
			Name:       localInterfaces[0].Name,
			IsPrimary:  false,
			IPSubnets:  []IPSubent{{Prefix: "2.0.0.0/12"}},
		},
	}

	err = populateAddressSpace(local, sdnInterfaces, localInterfaces)
	if err != nil {
		t.Fatalf("Error populating address space: %v", err)
	}

	if len(local.Pools) != 0 {
		t.Fatalf("Pool list has incorrect length. expected: %d, actual: %d", 0, len(local.Pools))
	}
}

func newLocalAddressSpace() *addressSpace {
	return &addressSpace{
		Id:    LocalDefaultAddressSpaceId,
		Scope: LocalScope,
		Pools: make(map[string]*addressPool),
	}
}

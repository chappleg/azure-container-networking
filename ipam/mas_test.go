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

	if runtime.GOOS == windows {
		if mas.filePath != defaultWindowsFilePath {
			t.Fatalf("default file path set incorrectly")
		}
	} else {
		if mas.filePath != defaultLinuxFilePath {
			t.Fatalf("default file path set incorrectly")
		}
	}
	if mas.name != "MAS" {
		t.Fatalf("mas source Name incorrect")
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

	correctInterfaces := &NetworkInterfaces{
		Interfaces: []Interface{
			{
				MacAddress: "000D3A6E1825",
				IsPrimary:  true,
				IPSubnets: []IPSubnet{
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

	local := &addressSpace{
		Id:    LocalDefaultAddressSpaceId,
		Scope: LocalScope,
		Pools: make(map[string]*addressPool),
	}

	sdnInterfaces := &NetworkInterfaces{
		Interfaces: []Interface{
			{
				MacAddress: "000D3A6E1825",
				IsPrimary:  true,
				IPSubnets: []IPSubnet{
					{
						Prefix: "1.0.0.0/12",
						IPAddresses: []IPAddress{
							{Address: "1.1.1.5", IsPrimary: true},
							{Address: "1.1.1.6", IsPrimary: false},
							{Address: "1.1.1.6", IsPrimary: false},
							{Address: "1.1.1.7", IsPrimary: false},
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

	if len(pool.Addresses) != 2 {
		t.Fatalf("Address list has incorrect length. expected: %d, actual: %d", 2, len(pool.Addresses))
	}

	_, ok = pool.Addresses["1.1.1.6"]
	if !ok {
		t.Fatal("Address 1.1.1.6 missing")
	}

	_, ok = pool.Addresses["1.1.1.7"]
	if !ok {
		t.Fatal("Address 1.1.1.7 missing")
	}
}

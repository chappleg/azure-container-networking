// Copyright 2017 Microsoft. All rights reserved.
// MIT License

package ipam

import (
	"encoding/json"
	"io/ioutil"
	"net"
	"runtime"
	"strings"

	"github.com/Azure/azure-container-networking/log"
)

const (
	defaultLinuxFilePath = "/etc/kubernetes/interfaces.json"
	defaultWindowsFilePath = `c:\k\interfaces.json`
)

// Microsoft Azure Stack IPAM configuration source.
type masSource struct {
	name       string
	sink       addressConfigSink
	fileLoaded bool
	filePath   string
}

// MAS host agent JSON object format.
type Interface struct {
	MacAddress string
	Name       string
	IsPrimary  bool
	IPSubnets  []IPSubent
}

type IPSubent struct {
	Prefix      string
	IPAddresses []IPAddress
}

type IPAddress struct {
	Address   string
	IsPrimary bool
}

// Creates the MAS source.
func newMasSource(options map[string]interface{}) (*masSource, error) {
	var filePath string
	if runtime.GOOS == "windows" {
		filePath = defaultWindowsFilePath
	} else {
		filePath = defaultLinuxFilePath
	}

	return &masSource{
		name: "MAS",
		filePath: filePath,
	}, nil
}

// Starts the MAS source.
func (source *masSource) start(sink addressConfigSink) error {
	source.sink = sink
	return nil
}

// Stops the MAS source.
func (source *masSource) stop() {
	source.sink = nil
	return
}

// Refreshes configuration.
func (source *masSource) refresh() error {

	if source.fileLoaded {
		return nil
	}

	// Query the list of local interfaces.
	localInterfaces, err := net.Interfaces()
	if err != nil {
		return err
	}

	// Query the list of Azure Network Interfaces
	sdnInterfaces, err := getSDNInterfaces(source.filePath)
	if err != nil {
		return err
	}

	// Configure the local default address space.
	local, err := source.sink.newAddressSpace(LocalDefaultAddressSpaceId, LocalScope)
	if err != nil {
		return err
	}

	err = populateAddressSpace(local, sdnInterfaces, localInterfaces)
	if err != nil {
		return err
	}

	// Set the local address space as active.
	err = source.sink.setAddressSpace(local)
	if err != nil {
		return err
	}

	source.fileLoaded = true

	return nil
}

func getSDNInterfaces(fileLocation string) ([]Interface, error) {
	data, err := ioutil.ReadFile(fileLocation)
	if err != nil {
		return nil, err
	}

	var interfaces []Interface
	err = json.Unmarshal(data, &interfaces)
	if err != nil {
		return nil, err
	}

	return interfaces, nil
}

func populateAddressSpace(localAddressSpace *addressSpace, sdnInterfaces []Interface, localInterfaces []net.Interface) error {

	//Find the interface with matching MacAddress or Name
	for _, sdnIf := range sdnInterfaces {
		ifName := ""

		for _, localIf := range localInterfaces {
			if (sdnIf.MacAddress == "" && sdnIf.Name == localIf.Name) || macAddressesEqual(sdnIf.MacAddress, localIf.HardwareAddr.String()) {
				ifName = localIf.Name
				break
			}
		}

		// Skip if interface is not found.
		if ifName == "" {
			log.Printf("[ipam] Failed to find interface with MAC address:%v or Name:%v.", sdnIf.MacAddress, sdnIf.Name)
			continue
		}

		// Prioritize secondary interfaces.
		priority := 0
		if !sdnIf.IsPrimary {
			priority = 1
		}

		for _, subnet := range sdnIf.IPSubnets {
			_, network, err := net.ParseCIDR(subnet.Prefix)
			if err != nil {
				log.Printf("[ipam] Failed to parse subnet:%v err:%v.", subnet.Prefix, err)
				continue
			}

			addressPool, err := localAddressSpace.newAddressPool(ifName, priority, network)
			if err != nil {
				log.Printf("[ipam] Failed to create pool:%v ifName:%v err:%v.", subnet, ifName, err)
				continue
			}

			// Add the IP addresses to the localAddressSpace address space.
			for _, ipAddr := range subnet.IPAddresses {
				// Primary addresses are reserved for the host.
				if ipAddr.IsPrimary {
					continue
				}

				address := net.ParseIP(ipAddr.Address)

				_, err = addressPool.newAddressRecord(&address)
				if err != nil {
					log.Printf("[ipam] Failed to create address:%v err:%v.", address, err)
					continue
				}
			}
		}
	}

	return nil
}

func macAddressesEqual(macAddress1 string, macAddress2 string) bool {
	macAddress1 = strings.ToLower(strings.Replace(macAddress1, ":", "", -1))
	macAddress2 = strings.ToLower(strings.Replace(macAddress2, ":", "", -1))

	return macAddress1 == macAddress2
}

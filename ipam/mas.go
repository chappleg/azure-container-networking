// Copyright 2017 Microsoft. All rights reserved.
// MIT License

package ipam

import (
	"encoding/json"
	"github.com/Azure/azure-container-networking/common"
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
	filePath, ok := options[common.OptIpamMASFilePath].(string)
	if !ok {
		if runtime.GOOS == "windows" {
			filePath = defaultWindowsFilePath
		} else {
			filePath = defaultLinuxFilePath
		}
	}

	return &masSource{
		name: "MAS",
		filePath: filePath,
	}, nil
}

// Starts the MAS source.
func (s *masSource) start(sink addressConfigSink) error {
	s.sink = sink
	return nil
}

// Stops the MAS source.
func (s *masSource) stop() {
	s.sink = nil
	return
}

// Refreshes configuration.
func (s *masSource) refresh() error {

	if s.fileLoaded {
		return nil
	}

	// Query the list of local interfaces.
	localInterfaces, err := net.Interfaces()
	if err != nil {
		return err
	}

	// Query the list of Azure Network Interfaces
	sdnInterfaces, err := getSDNInterfaces(s.filePath)
	if err != nil {
		return err
	}

	// Configure the local default address space.
	local, err := s.sink.newAddressSpace(LocalDefaultAddressSpaceId, LocalScope)
	if err != nil {
		return err
	}

	err = populateAddressSpace(local, sdnInterfaces, localInterfaces)
	if err != nil {
		return err
	}

	// Set the local address space as active.
	err = s.sink.setAddressSpace(local)
	if err != nil {
		return err
	}

	s.fileLoaded = true

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

func populateAddressSpace(local *addressSpace, sdnInterfaces []Interface, localInterfaces []net.Interface) error {

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

			addressPool, err := local.newAddressPool(ifName, priority, network)
			if err != nil {
				log.Printf("[ipam] Failed to create pool:%v ifName:%v err:%v.", subnet, ifName, err)
				continue
			}

			// Add the IP addresses to the local address space.
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

func macAddressesEqual(a1 string, a2 string) bool {
	a1 = strings.ToLower(strings.Replace(a1, ":", "", -1))
	a2 = strings.ToLower(strings.Replace(a2, ":", "", -1))

	return a1 == a2
}

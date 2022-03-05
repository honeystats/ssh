package main

import "math/rand"

func randSourceIP() string {
	ips := []string{
		"82.67.74.30",
		"55.159.212.43",
		"108.218.89.226",
		"189.65.42.171",
		"62.218.183.66",
		"210.116.94.157",
		"80.243.180.223",
		"169.44.232.173",
		"232.117.72.103",
		"242.14.158.127",
		"14.209.62.41",
		"4.110.11.42",
		"135.235.149.26",
		"93.60.177.34",
		"145.121.235.122",
		"170.68.154.171",
		"206.234.141.195",
		"179.22.18.176",
		"178.35.233.119",
		"145.156.239.238",
		"192.114.2.154",
		"212.36.131.210",
		"252.185.209.0",
		"238.49.69.205",
	}
	lenIps := len(ips)
	randIndex := rand.Intn(lenIps)
	return ips[randIndex]
}
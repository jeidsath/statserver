package main

import (
	"fmt"
	"testing"
)

func ip4ToInt64(a, b, c, d int) int64 {
	return int64(256*256*256*a + 256*256*b + 256*c + d)
}

func TestReduce(t *testing.T) {
	ip := ip4ToInt64(107, 92, 8, 5)
	val := ip4ToInt64(107, 92, 8, 5/16)
	if reduceIp(ip) != val {
		fmt.Printf("Reduce of %v was %v, should be %v", ip, reduceIp(ip), val)
		t.Fail()
	}
}

func TestStringIp(t *testing.T) {
	ip := ip4ToInt64(123, 45, 12, 1)
	val := "123.45.12.1"
	if stringIp(ip) != val {
		fmt.Printf("String of %v was %v, should be %v", ip, stringIp(ip), val)
	}
}

func TestFunctional(t *testing.T) {
	sha := "0fe3fa2fa0869e5100e24ede99f6daf2fc8a30cfd3a10e9a8e17b8926fc445ce"
	ips := []int64{
		ip4ToInt64(192, 160, 0, 1),
		ip4ToInt64(192, 160, 0, 2),
		ip4ToInt64(192, 160, 0, 3),
		ip4ToInt64(192, 160, 0, 4),
		ip4ToInt64(10, 0, 0, 1),
	}

	for _, ip := range ips {
		storeShaIp(sha, ip)
	}

	expect := "{\"count\":5,\"good_ips\":[\"192.160.0.1\",\"192.160.0.2\",\"192.160.0.3\",\"192.160.0.4\"],\"bad_ips\":[\"10.0.0.1\"]}"

	output, err := jsonForApp(sha)
	if output != expect {
		fmt.Printf("Expect %v but got %v\n", expect, output)
		t.Fail()
	}
	if err != nil {
		fmt.Printf(err.Error())
		t.Fail()
	}
}

package zone

import (
	"os"
	"strconv"
	"strings"
	"testing"
)

func TestNewK8sSite(t *testing.T) {
	domain, port, path := "", 0, ""

	for i := range os.Args {

		args := strings.Split(os.Args[i], " ")

		for _, v := range args {
			parts := strings.SplitN(v, "=", 2)

			if len(parts) != 2 {
				continue
			}

			switch parts[0] {
			case "domain":
				domain = parts[1]

			case "port":
				n, err := strconv.Atoi(parts[1])
				if err != nil {
					t.Error(v, err)
				}

				port = n

			case "path":
				path = parts[1]

			default:
			}
		}
	}

	if domain == "" && port == 0 && path == "" {
		t.Skip(os.Args)
	}

	site := NewK8sSite(domain, domain, "", path, port)

	err := site.initSite()
	if err != nil {
		t.Error(domain, port, path, err)
	}

	iface, err := site.SiteInterface()
	if err != nil {
		t.Fatal(err)
	}

	err = iface.Connected()
	if err != nil {
		t.Fatal(err)
	}
}

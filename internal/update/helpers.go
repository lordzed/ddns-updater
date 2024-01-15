package update

import (
	"fmt"
	"net/netip"
)

func (r *Runner) logDebugNoLookupSkip(hostname, ipKind string, lastIP, ip netip.Addr) {
	r.logger.Debug(fmt.Sprintf("Last %s address stored for %s is %s and your %s address"+
		" is %s, skipping update", ipKind, hostname, lastIP, ipKind, ip))
}

func (r *Runner) logInfoNoLookupUpdate(hostname, ipKind string, lastIP, ip netip.Addr) {
	r.logger.Info(fmt.Sprintf("Last %s address stored for %s is %s and your %s address is %s",
		ipKind, hostname, lastIP, ipKind, ip))
}

func (r *Runner) logDebugLookupSkip(hostname, ipKind string, recordIP, ip netip.Addr) {
	r.logger.Debug(fmt.Sprintf("%s address of %s is %s and your %s address"+
		" is %s, skipping update", ipKind, hostname, recordIP, ipKind, ip))
}

func (r *Runner) logInfoLookupUpdate(hostname, ipKind string, recordIP, ip netip.Addr) {
	r.logger.Info(fmt.Sprintf("%s address of %s is %s and your %s address  is %s",
		ipKind, hostname, recordIP, ipKind, ip))
}

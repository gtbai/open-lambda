package cache

/*
#include <stdio.h>
#include <stdlib.h>
#include <errno.h>
#include <string.h>
#include <fcntl.h>
#include <unistd.h>
#include <sys/types.h>
#include <sys/stat.h>
#include <sys/socket.h>
#include <sys/un.h>

char errmsg[1024];

int
sendfd(int s, int fd) {
	char buf[1];
	struct iovec iov;
	struct msghdr msg;
	struct cmsghdr *cmsg;
	int n;
	char cms[CMSG_SPACE(sizeof(int))];

	buf[0] = 0;
	iov.iov_base = buf;
	iov.iov_len = 1;

	memset(&msg, 0, sizeof msg);
	msg.msg_iov = &iov;
	msg.msg_iovlen = 1;
	msg.msg_control = (caddr_t)cms;
	msg.msg_controllen = CMSG_LEN(sizeof(int));

	cmsg = CMSG_FIRSTHDR(&msg);
	cmsg->cmsg_len = CMSG_LEN(sizeof(int));
	cmsg->cmsg_level = SOL_SOCKET;
	cmsg->cmsg_type = SCM_RIGHTS;
	memmove(CMSG_DATA(cmsg), &fd, sizeof(int));

	if((n = sendmsg(s, &msg, 0)) != iov.iov_len) {
        return -1;
    }

	return 0;
}

const char*
sendFds(char *sockPath, char *pid, char *rootdir, char *pkgs) {
    char *path;
    int k;

    // Namespaces to be merged (all but 'user') - MUST merge 'mnt' last

    const int NUM_NS = 6;
    int nsfds[NUM_NS];
    const char *ns[NUM_NS];
    ns[0] = "cgroup";
    ns[1] = "ipc";
    ns[2] = "uts";
    ns[3] = "net";
    ns[4] = "pid";
    ns[5] = "mnt";

    // Get fds for all namespaces.

    for(k = 0; k < NUM_NS; k++) {
        path = (char*)malloc(10+strlen(pid)+strlen(ns[k]));
        sprintf(path, "/proc/%s/ns/%s", pid, ns[k]);

        nsfds[k] = open(path, O_RDONLY);
        if (nsfds[k] == -1) {
            sprintf(errmsg, "open: %s\n", strerror(errno));
            return errmsg;
        }
    }

    // Connect to server via socket.

    int s, len, ret;
    struct sockaddr_un remote;

    if ((s = socket(AF_UNIX, SOCK_STREAM, 0)) == -1) {
        sprintf(errmsg, "socket: %s\n", strerror(errno));
        return errmsg;
    }

    remote.sun_family = AF_UNIX;
    strcpy(remote.sun_path, sockPath);
    len = strlen(remote.sun_path) + sizeof(remote.sun_family);
    if (connect(s, (struct sockaddr *)&remote, len) == -1) {
        sprintf(errmsg, "connect: %s\n", strerror(errno));
        return errmsg;
    }

    // Send fds to server.

    for(k = 0; k < NUM_NS; k++) {
        if (sendfd(s, nsfds[k]) == -1) {
            sprintf(errmsg, "sendfd: %s\n", strerror(errno));
            return errmsg;
        }
    }

    // Send root directory string to server.

    int rootbuflen = 500;
    printf("Sending package string.\n");
    if(send(s, rootdir, rootbuflen, 0) == -1) {
        sprintf(errmsg, "send rootdir: %s\n", strerror(errno));
        return errmsg;
    }

    // Send package string to server.

    int pkgbuflen = 5000;
    if(send(s, pkgs, pkgbuflen, 0) == -1) {
        sprintf(errmsg, "send pkgs: %s\n", strerror(errno));
        return errmsg;
    }

    // Receive spawned PID from server.

    static char buf[50];

    if((len = recv(s, buf, 50, 0)) == -1) {
        sprintf(errmsg, "recv: %s\n", strerror(errno));
        return errmsg;
    }

    if(close(s) == -1) {
        sprintf(errmsg, "close: %s\n", strerror(errno));
        return errmsg;
    }

    // Close fds for all namespaces.

    for(k = 0; k < NUM_NS; k++) {
        if(close(nsfds[k]) == -1) {
            sprintf(errmsg, "close: %s\n", strerror(errno));
            return errmsg;
        }
    }

    return buf;
}
*/
import "C"

import (
	"errors"
	"fmt"
	"log"
	"strings"

	"github.com/open-lambda/open-lambda/worker/benchmarker"
)

/*
 * Send the namespace file descriptors for the targetPid process
 * and the passed package list to a lambda server listening on the
 * unix socket at sockPath.
 *
 * The packages in pkgList are assumed to be whitespace-delimited.
 *
 * Returns the PID of the spawned process upon success.
 */

func forkRequest(sockPath, targetPid, rootDir string, pkgList []string, handler bool) (pid string, err error) {
	b := benchmarker.GetBenchmarker()
	var t *benchmarker.Timer
	if b != nil {
		t = b.CreateTimer("send fds", "us")
	}

	if t != nil {
		t.Start()
	}

	var signal string
	if handler {
		signal = "serve"
	} else {
		signal = "cache"
	}

	log.Printf("FORKREQUEST PATH: %s\n", sockPath)
	log.Printf("FORKREQUEST PID: %s\n", targetPid)

	pkgStr := strings.Join(append(pkgList, signal), " ")

	csock := C.CString(sockPath)
	cpid := C.CString(targetPid)
	croot := C.CString(rootDir)
	cpkgs := C.CString(pkgStr)

	ret, err := C.sendFds(csock, cpid, croot, cpkgs)
	if t != nil {
		if err == nil {
			t.End()
		} else {
			t.Error("Send Fds Failed")
		}
	}
	pid = C.GoString(ret)
	if err != nil {
		return "", errors.New(fmt.Sprintf("sendFds: %s", pid))
	}

	return pid, nil
}
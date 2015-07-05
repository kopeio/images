package user

import (
	"errors"
	"github.com/kopeio/kope/chained"
	"os"
	"os/user"
	"strconv"
)

type User struct {
	Uid  int
	Gid  int
	User *user.User
}

func Find(username string) (*User, error) {
	u, err := user.Lookup(username)
	if err != nil {
		return nil, chained.Error(err, "error looking up user: "+username)
	}
	if u == nil {
		return nil, errors.New("cannot find user: " + username)
	}
	uid, err := strconv.Atoi(u.Uid)
	if err != nil {
		return nil, chained.Error(err, "error parsing uid for user: "+username)
	}
	gid, err := strconv.Atoi(u.Gid)
	if err != nil {
		return nil, chained.Error(err, "error parsing gid for user: "+username)
	}
	s := &User{}
	s.Uid = uid
	s.Gid = gid
	s.User = u
	return s, nil
}

func (u *User) Chown(f string) error {
	err := os.Chown(f, u.Uid, u.Gid)
	if err != nil {
		return chained.Error(err, "error doing chown on: ", f)
	}
	return nil
}

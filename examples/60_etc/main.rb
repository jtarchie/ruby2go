# rbs_inline: enabled
# args: --seed 1
require "etc"
require "minitest/autorun"

# Etc via os/user and runtime; machine-specific values are checked structurally, not against literal expectations.

class EtcSystemTest < Minitest::Test
  def test_getlogin_nprocessors_and_systmpdir
    login = Etc.getlogin
    assert_equal true, login.nil? || login.is_a?(String)
    assert_equal true, Etc.nprocessors.is_a?(Integer) && Etc.nprocessors > 0
    tmpdir = Etc.systmpdir
    assert_equal true, tmpdir.is_a?(String) && !tmpdir.empty?
  end
end

class EtcPasswdTest < Minitest::Test
  MEMBERS = [:name, :passwd, :uid, :gid, :gecos, :dir, :shell, :change, :uclass, :expire] #: Array[Symbol]

  def test_getpwuid_describes_the_current_user
    pw = Etc.getpwuid
    assert_equal true, pw.is_a?(Etc::Passwd)
    assert_equal true, pw.name.is_a?(String) && !pw.name.empty?
    assert_equal true, pw.uid.is_a?(Integer) && pw.uid >= 0
    assert_equal true, pw.gid.is_a?(Integer) && pw.gid >= 0
    assert_equal true, pw.dir.is_a?(String) && !pw.dir.empty?
    assert_equal true, pw.gecos.is_a?(String)
  end

  def test_lookups_by_uid_and_name_round_trip
    pw = Etc.getpwuid
    by_uid = Etc.getpwuid(pw.uid)
    assert_equal true, by_uid.name == pw.name && by_uid.uid == pw.uid
    by_name = Etc.getpwnam(pw.name)
    assert_equal true, by_name.uid == pw.uid && by_name.dir == pw.dir
  end

  def test_passwd_is_a_struct
    pw = Etc.getpwuid
    assert_equal MEMBERS, pw.members
    assert_equal MEMBERS, pw.to_h.keys
    assert_equal MEMBERS, Etc::Passwd.members
  end

  def test_unknown_users_raise_argument_error
    e = assert_raises(ArgumentError) { Etc.getpwuid(999_999_999) }
    assert_equal "can't find user for 999999999", e.message
    e = assert_raises(ArgumentError) { Etc.getpwnam("no_such_rb2go_user") }
    assert_equal "can't find user for no_such_rb2go_user", e.message
  end

  # Trailing fields left out of Passwd.new are nil.
  def test_passwd_new_and_inspect
    alice = Etc::Passwd.new("alice", "x", 1000, 1000, "Alice Example", "/home/alice", "/bin/bash", 0, "", 0)
    assert_equal '#<struct Etc::Passwd name="alice", passwd="x", uid=1000, gid=1000, gecos="Alice Example", dir="/home/alice", shell="/bin/bash", change=0, uclass="", expire=0>', alice.inspect
    bob = Etc::Passwd.new("bob", "x", 1001, 1001, "Bob Example", "/home/bob")
    assert_equal '#<struct Etc::Passwd name="bob", passwd="x", uid=1001, gid=1001, gecos="Bob Example", dir="/home/bob", shell=nil, change=nil, uclass=nil, expire=nil>', bob.inspect
  end
end

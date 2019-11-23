Build Oracle Docker Image
```bash
$ git clone git@github.com:oracle/docker-images.git
$ cd docker-images/OracleDatabase/SingleInstance
$ mv /path/to/oracle_src ./19.3.0/
$ ./buildDockerImage.sh -v 19.3.0 -e -i
```

Build mattn/go-oci8
```bash
$ export LD_LIBRARY_PATH=/Users/$(whoami)/Downloads/instantclient_19_3
$ export PKG_CONFIG_PATH=/Users/$(whoami)/Downloads/instantclient_19_3
```

* Download instant client **sdk**
* setup oci8.pc
```
prefixdir=/path/to/instantclient_19_3/
libdir=${prefixdir}
includedir=${prefixdir}/sdk/include

Name: OCI
Description: Oracle database driver
Version: 19.3
Libs: -L${libdir} -lclntsh
Cflags: -I${includedir}
```
* set blackfriday version to prevent error
```toml
[[override]]
  name = "github.com/russross/blackfriday"
  version = "1.5.2"
```

sqlplus on Docker
```bash
$ export NLS_LANG=Japanese_Japan.AL32UTF8
$ sqlplus system/Oracle19@localhost/ORCLCDB
```

if [ "X${VELERO_VERSION}X" == "XX" ]
then
  echo -n "* VELERO version to use: "
  read -r VELERO_VERSION
  echo
fi

TEMPDIR=$(mktemp -d)
cd "${TEMPDIR}" || exit

if ! curl -L "https://github.com/vmware-tanzu/velero/releases/download/${VELERO_VERSION}/velero-${VELERO_VERSION}-linux-amd64.tar.gz" --output velero.tar.gz
then
  echo "Can not download velero"
  exit 1
fi

if ! tar xzf velero.tar.gz
then
  echo "Can not unpack velero"
  exit 1
fi

if ! mv "velero-${VELERO_VERSION}-linux-amd64/velero" /home/cloudcontrol/bin
then
  echo "Can not move velero binary"
  exit 1
fi

cd - || exit
rm -rf "${TEMPDIR}"

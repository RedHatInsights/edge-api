#!/bin/bash -x
: <<'_usage_'

KS_FQPATH: Fully qualified path to kickstart file
ISO_IN: Path to source ISO
ISO_OUT: Path to target ISO
WORKDIR: Path to the directory the source ISO will be exploded into.

./fleetkick.sh kickstart-stage.ks \
/var/lib/libvirt/images/composer-api-d83040bf-b410-4317-9121-1ee7bd726ac1-rhel84-boot.iso \
/var/lib/libvirt/images/ksinjected/composer-api-d83040bf-b410-4317-9121-1ee7bd726ac1-rhel84-boot_notefi.iso \
/home/root/edgeiso/download98a

_usage_

KS_FQPATH=$1
ISO_IN=$2
ISO_OUT=$3
WORKDIR=$4

[[ -e "$KS_FQPATH" ]] || (echo "ERROR: no kickstart file" && exit 1)
[[ -e "$ISO_IN" ]] || (echo "ERROR: no source ISO file" && exit 1)

ISOCFG="${WORKDIR}/isolinux/isolinux.cfg"
EFICFG="${WORKDIR}/EFI/BOOT/grub.cfg"
EFI_DIR="${WORKDIR}/EFI/BOOT"
EFI_IMAGEPATH="${WORKDIR}/images/efiboot.img"
KSFILE="fleet.ks"


validate_kickstart() {
    KS=$1

    echo "Validating kickstart $KS"

    [[ -e "$KS" ]] && ksvalidator -v RHEL8 "$KS" || (echo "ERROR: no kickstart file" && exit 1)
}


get_iso_volid() {
    ISO=$1

    [[ -e "$ISO" ]] && volid=`isoinfo -d -i "$ISO" | grep "Volume id:" | awk -F': ' '{print $2}'` \
        || (echo "ERROR: no $ISO file"; exit 1)
    echo $volid
}


explode_iso() {
    ISO=$1
    DIR=$2

    # can also be accomplished with 7z x $ISO -o${DIR}
    [[ -e "$ISO" ]] && xorriso -osirrox on -indev "$ISO" -extract / $DIR
}


insert_kickstart() {
    KS=$1
    DIR=$2

    echo "Copying ks file $KS to $DIR"
    [[ -e $KS ]] && cp $KS $DIR || (echo "ERROR: no kickstart file" && exit 1)
}


edit_isolinux() {
    CONFIG=$1
    VOLID=$2
    KICKFILE=$3

    [[ -e $CONFIG ]] && file $CONFIG || (echo "ERROR: no $CONFIG file" && exit 1)
    # Add inst.stage2 if missing (see https://bugzilla.redhat.com/show_bug.cgi?id=2152192)
    sed -i "/rescue/n;/inst.stage2/n;/LABEL=${VOLID}/ s/$/ inst.stage2=hd:LABEL=${VOLID}/g" $CONFIG
    # Remove an existing inst.ks instruction
    sed -i "/rescue/n;/LABEL=${VOLID}/ s/\<inst.ks[^ ]*//g" $CONFIG
    # Replace an existing inst.ks instruction
    sed -i "/rescue/n;/LABEL=${VOLID}/ s/\<inst.ks[^ ]*/inst.ks=hd:LABEL=${VOLID}:\/${KICKFILE}/g" $CONFIG
    # Inject an inst.ks instruction
    sed -i "/inst.ks=/n;/rescue/n;/LABEL=${VOLID}/ s/$/ inst.ks=hd:LABEL=${VOLID}:\/${KICKFILE}/g" $CONFIG
    grep $VOLID $CONFIG
}


edit_efiboot() {
    CONFIG=$1
    VOLID=$2
    KICKFILE=$3

    [[ -e $CONFIG ]] && file $CONFIG || (echo "ERROR: no $CONFIG file" && exit 1)
    # Add inst.stage2 if missing (see https://bugzilla.redhat.com/show_bug.cgi?id=2152192)
    sed -i "/rescue/n;/inst.stage2/n;/LABEL=${VOLID}/ s/$/ inst.stage2=hd:LABEL=${VOLID}/g" $CONFIG
    # Remove an existing inst.ks instruction
    sed -i "/rescue/n;/LABEL=${VOLID}/ s/\<inst.ks[^ ]*//g" $CONFIG
    # Replace an existing inst.ks instruction
    sed -i "/rescue/n;/LABEL=${VOLID}/ s/\<inst.ks[^ ]*/inst.ks=hd:LABEL=${VOLID}:\/${KICKFILE}/g" $CONFIG
    # Inject an inst.ks instruction
    sed -i "/inst.ks=/n;/rescue/n;/LABEL=${VOLID}/ s/$/ inst.ks=hd:LABEL=${VOLID}:\/${KICKFILE}/g" $CONFIG
    grep $VOLID $CONFIG
}


modify_efiboot_image() {
    CONFIG=$1
    IMAGE=$2

    mtype -i $IMAGE ::EFI/BOOT/grub.cfg | grep linuxefi
    mcopy -o -i $IMAGE $CONFIG ::EFI/BOOT/grub.cfg
    mtype -i $IMAGE ::EFI/BOOT/grub.cfg | grep linuxefi
}


regen_efi_image() {
    EFI_IN=$1
    EFI_OUT=$2

    [[ -e $EFI_IN ]] && mkefiboot --label=ANACONDA --debug $EFI_IN $EFI_OUT \
         || (echo "ERROR: no EFI/BOOT dir"; exit 1)
}


make_the_iso() {
    MKISODIR=$1
    ISOPATH=$2
    VOLUME_ID=$3

    cd $MKISODIR

    genisoimage -o "$ISOPATH" -R -J \
-V "${VOLUME_ID}" \
-A "${VOLUME_ID}" \
-volset "${VOLUME_ID}" \
-b isolinux/isolinux.bin \
-c isolinux/boot.cat \
-boot-load-size 4 \
-boot-info-table \
-no-emul-boot \
-verbose \
-debug \
-eltorito-alt-boot \
-e images/efiboot.img -no-emul-boot .
}

hybridify() {
    ISOPATH=$1

    [[ -e "$ISOPATH" ]] && isohybrid --uefi "$ISOPATH" || (echo "${ISOPATH} does not exist"; exit 1)
}

implant_md5() {
    ISOPATH=$1

    [[ -e "$ISOPATH" ]] && implantisomd5 "$ISOPATH" || (echo "${ISOPATH} does not exist"; exit 1)
}


echo "KS_FQPATH: ${KS_FQPATH}"
echo "ISO IN: ${ISO_IN}"
echo "ISO OUT: ${ISO_OUT}"
echo "WORKDIR: ${WORKDIR}"

echo "ISOCFG: ${ISOCFG}"
echo "EFICFG: ${EFICFG}"
echo "KSFILE: ${KSFILE}"

validate_kickstart "$KS_FQPATH"
VOLID=$(get_iso_volid "$ISO_IN")
echo "VOLID: $VOLID"
explode_iso "$ISO_IN" $WORKDIR
insert_kickstart "$KS_FQPATH" "${WORKDIR}/${KSFILE}"
edit_isolinux $ISOCFG $VOLID "$KSFILE"
edit_efiboot $EFICFG $VOLID "$KSFILE"
modify_efiboot_image $EFICFG $EFI_IMAGEPATH
#regen_efi_image $EFI_DIR $EFI_IMAGEPATH
make_the_iso $WORKDIR "$ISO_OUT" $VOLID
hybridify "$ISO_OUT"
implant_md5 "$ISO_OUT"

echo
echo "New ISO: $ISO_OUT"

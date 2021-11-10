###################################
###### Build the FDO package ######
###################################
FROM quay.io/centos/centos AS base
RUN yum install -y cargo git-core openssl-devel
RUN git clone https://github.com/fedora-iot/fido-device-onboard-rs.git
RUN cd fido-device-onboard-rs && cargo build --release

##############################################
###### Build libfdo-data test container ######
##############################################
FROM registry.access.redhat.com/ubi8/ubi-minimal AS libfdo-data
ENV LD_LIBRARY_PATH /usr/local/lib
COPY --from=base /fido-device-onboard-rs/target/release/libfdo_data.so /usr/local/lib/
COPY --from=base /fido-device-onboard-rs/libfdo-data/fdo_data.h /usr/local/include/
COPY --from=base /fido-device-onboard-rs/libfdo-data/test_assets/testdevice1.ov ./testdevice1.ov

RUN microdnf install git go -y
RUN echo 'if [[ ! -d "edge-api" ]];then git clone https://github.com/RedHatInsights/edge-api.git; fi && \ 
    cd edge-api && \
    cp ../testdevice1.ov pkg/services/ownershipvoucher/ && \
    go test ./pkg/services/ownershipvoucher/...' > test_ov.sh
RUN chmod +x ./test_ov.sh
CMD [ "sh", "./test_ov.sh" ]

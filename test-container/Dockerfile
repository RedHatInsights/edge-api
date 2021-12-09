###################################
###### Build the FDO package ######
###################################
FROM registry.access.redhat.com/ubi8/ubi AS builder
RUN yum install -y cargo git-core openssl-devel
RUN git clone https://github.com/fedora-iot/fido-device-onboard-rs.git
RUN cd fido-device-onboard-rs && cargo build --lib

##############################################
###### Build libfdo-data test container ######
##############################################
FROM registry.access.redhat.com/ubi8/ubi-minimal AS libfdo-data
RUN microdnf install make go -y

RUN echo -e 'if [[ ! -d "edge-api" ]];then \n\
    microdnf install git -y && git clone -b "${GIT_UPSTREAM_BRANCH}" "${GIT_UPSTREAM_REMOTE}"; fi\n \ 
    make -C edge-api coverage' > test.sh
RUN chmod +x ./test.sh

ENV LD_LIBRARY_PATH /usr/local/lib
COPY --from=builder /fido-device-onboard-rs/target/debug/libfdo_data.so ${LD_LIBRARY_PATH}/libfdo_data.so.0
COPY --from=builder /fido-device-onboard-rs/libfdo-data/fdo_data.h /usr/local/include/
COPY --from=builder /fido-device-onboard-rs/libfdo-data/test_assets/testdevice1.ov ./testdevice1.ov

ARG GIT_BRANCH="main"
ARG GIT_REMOTE="https://github.com/RedHatInsights/edge-api.git"
ENV GIT_UPSTREAM_REMOTE ${GIT_REMOTE}
ENV GIT_UPSTREAM_BRANCH ${GIT_BRANCH}

CMD [ "sh", "./test.sh" ]

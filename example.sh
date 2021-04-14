go build . 

trap 'pkill wstunnel' SIGINT SIGTERM EXIT 

./wstunnel client \
  -port-forward '6665:6789' \
  -server 'http://100.160.50.104:8080' \
  -proxy 'http://user:password@111.106.10.165:9090' \
  -password password \
  -debug \
  & 
P1=$!

./wstunnel server \
  -port 8080 \
  -password password \
  -debug \
P2=$!

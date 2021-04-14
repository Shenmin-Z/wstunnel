go build . 

trap 'pkill wstunnel' SIGINT SIGTERM EXIT 

./wstunnel client \
  -f '6665:6789' \
  -s 'http://100.160.50.104:8080' \
  -p 'http://user:password@111.106.10.165:9090' \
  -w password \
  -d \
  & 
P1=$!

./wstunnel server \
  -p 8080 \
  -w password \
  -d \
P2=$!

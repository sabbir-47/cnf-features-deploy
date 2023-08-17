#!/bin/bash

start=$(date +%s)

for i in {1..300}
do
   make test
done

end=$(date +%s)

echo "----------------------------------"
echo "----------------------------------"
echo "Elapsed Time: $(($end-$start)) seconds"

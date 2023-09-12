search_string=fluxd/chain
replace_string=sdk-go/chain
files=$(find ./chain -type f)
for file in $files; do
  sed -i '' -e "s|$search_string|$replace_string|g" $file
done

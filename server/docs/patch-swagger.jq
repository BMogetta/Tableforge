.[0].definitions as $defs |
.[1] as $patch |
.[0] | .definitions |= with_entries(
    .key as $name |
    if $patch[$name] then
        .value.required = $patch[$name]
    else . end
)
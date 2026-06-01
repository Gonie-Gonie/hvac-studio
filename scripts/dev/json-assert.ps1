function Test-IsJsonNumber {
  param([object]$Value)

  return $Value -is [byte] `
    -or $Value -is [int16] `
    -or $Value -is [int32] `
    -or $Value -is [int64] `
    -or $Value -is [single] `
    -or $Value -is [double] `
    -or $Value -is [decimal]
}

function Assert-JsonSubset {
  param(
    [Parameter(Mandatory = $true)][AllowNull()][object]$Expected,
    [Parameter(Mandatory = $true)][AllowNull()][object]$Actual,
    [Parameter(Mandatory = $true)][string]$Path
  )

  if ($null -eq $Expected) {
    if ($null -ne $Actual) {
      throw "$Path expected null, got $Actual"
    }
    return
  }

  if ($Expected -is [System.Management.Automation.PSCustomObject]) {
    if ($Actual -isnot [System.Management.Automation.PSCustomObject]) {
      $ActualType = if ($null -eq $Actual) { 'null' } else { $Actual.GetType().Name }
      throw "$Path expected object, got $ActualType"
    }

    foreach ($Property in $Expected.PSObject.Properties) {
      $ActualProperty = $Actual.PSObject.Properties[$Property.Name]
      if ($null -eq $ActualProperty) {
        throw "$Path missing property: $($Property.Name)"
      }
      Assert-JsonSubset -Expected $Property.Value -Actual $ActualProperty.Value -Path "$Path.$($Property.Name)"
    }
    return
  }

  if ($Expected -is [System.Array]) {
    if ($Actual -isnot [System.Array]) {
      $ActualType = if ($null -eq $Actual) { 'null' } else { $Actual.GetType().Name }
      throw "$Path expected array, got $ActualType"
    }
    if ($Expected.Count -ne $Actual.Count) {
      throw "$Path expected array length $($Expected.Count), got $($Actual.Count)"
    }
    for ($Index = 0; $Index -lt $Expected.Count; $Index++) {
      Assert-JsonSubset -Expected $Expected[$Index] -Actual $Actual[$Index] -Path "$Path[$Index]"
    }
    return
  }

  if ((Test-IsJsonNumber $Expected) -and (Test-IsJsonNumber $Actual)) {
    $Delta = [Math]::Abs([double]$Expected - [double]$Actual)
    if ($Delta -gt 1e-9) {
      throw "$Path expected $Expected, got $Actual"
    }
    return
  }

  if ($Expected -ne $Actual) {
    throw "$Path expected $Expected, got $Actual"
  }
}


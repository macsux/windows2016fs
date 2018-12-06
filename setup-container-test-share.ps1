param (
    [string]$shareName,
    [string]$shareSourcePath,
    [bool]$add,
    [bool]$remove
)

if ($remove) {
    Remove-SmbShare -Name $shareName -Force 
}

if ($add) {
	New-SmbShare -Name $shareName -Path $shareSourcePath -ErrorAction Stop
}

Get-SmbShare

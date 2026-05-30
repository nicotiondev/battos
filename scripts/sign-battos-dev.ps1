param(
    [string]$ExePath = "$env:USERPROFILE\bin\battos.exe",
    [string]$Subject = "CN=Nicotion.dev BattOS Dev Code Signing",
    [string]$CertificatePath = ".\data\certs\battos-dev-code-signing.cer",
    [switch]$TrustPublisher = $true,
    [switch]$TrustRoot
)

$ErrorActionPreference = "Stop"

if (-not (Test-Path -LiteralPath $ExePath)) {
    throw "No existe el binario a firmar: $ExePath"
}

$cert = Get-ChildItem Cert:\CurrentUser\My -CodeSigningCert |
    Where-Object { $_.Subject -eq $Subject } |
    Sort-Object NotAfter -Descending |
    Select-Object -First 1

if (-not $cert) {
    $cert = New-SelfSignedCertificate `
        -Type CodeSigningCert `
        -Subject $Subject `
        -CertStoreLocation Cert:\CurrentUser\My `
        -KeyUsage DigitalSignature `
        -KeyExportPolicy Exportable `
        -NotAfter (Get-Date).AddYears(3)
}

if ($CertificatePath) {
    $certDir = Split-Path -Parent $CertificatePath
    if ($certDir -and -not (Test-Path -LiteralPath $certDir)) {
        New-Item -ItemType Directory -Path $certDir | Out-Null
    }
    Export-Certificate -Cert $cert -FilePath $CertificatePath -Force | Out-Null
}

if ($TrustPublisher) {
    $publisherStore = New-Object System.Security.Cryptography.X509Certificates.X509Store("TrustedPublisher", "CurrentUser")
    $publisherStore.Open("ReadWrite")
    try {
        $publisherStore.Add($cert)
    }
    finally {
        $publisherStore.Close()
    }
}

if ($TrustRoot) {
    $rootStore = New-Object System.Security.Cryptography.X509Certificates.X509Store("Root", "CurrentUser")
    $rootStore.Open("ReadWrite")
    try {
        $rootStore.Add($cert)
    }
    finally {
        $rootStore.Close()
    }
}

$signature = Set-AuthenticodeSignature -FilePath $ExePath -Certificate $cert
if ($signature.Status -ne "Valid" -and $signature.Status -ne "UnknownError") {
    $signature | Format-List Status,StatusMessage,Path,SignerCertificate
    throw "No se pudo firmar $ExePath"
}

Write-Host "Certificado:" $cert.Subject
Write-Host "Thumbprint:" $cert.Thumbprint
Write-Host "Exportado:" (Resolve-Path -LiteralPath $CertificatePath)
Write-Host "TrustedPublisher:" $TrustPublisher
Write-Host "TrustedRoot:" $TrustRoot

Get-AuthenticodeSignature -LiteralPath $ExePath |
    Format-List Status,StatusMessage,Path,SignerCertificate

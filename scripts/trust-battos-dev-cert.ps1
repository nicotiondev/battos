param(
    [string]$CertificatePath = ".\data\certs\battos-dev-code-signing.cer",
    [switch]$TrustRoot
)

$ErrorActionPreference = "Stop"

if (-not (Test-Path -LiteralPath $CertificatePath)) {
    throw "No existe el certificado publico: $CertificatePath"
}

$cert = New-Object System.Security.Cryptography.X509Certificates.X509Certificate2($CertificatePath)

$publisherStore = New-Object System.Security.Cryptography.X509Certificates.X509Store("TrustedPublisher", "CurrentUser")
$publisherStore.Open("ReadWrite")
try {
    $publisherStore.Add($cert)
}
finally {
    $publisherStore.Close()
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

Write-Host "Certificado confiado:" $cert.Subject
Write-Host "Thumbprint:" $cert.Thumbprint
Write-Host "TrustedPublisher: True"
Write-Host "TrustedRoot:" $TrustRoot

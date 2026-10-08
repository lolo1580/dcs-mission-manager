; DCS Manager custom per-user setup. Inno Setup 6.7+.
#ifndef AppVersion
  #define AppVersion "1.0.0-beta.5"
#endif
#ifndef StageDir
  #define StageDir "..\dist\staging"
#endif
#ifndef OutputPath
  #define OutputPath "..\dist"
#endif
[Setup]
#ifdef TestBuild
AppId={{4702E2AA-9F0C-47F0-AF11-8EBDF30A08B2}
AppName=DCS Manager Setup Test
#else
AppId={{7AB2C798-84D8-4E39-9E70-FCE89A30D640}
AppName=DCS Manager
#endif
AppVersion={#AppVersion}
AppPublisher=Laurent (lolo1580)
AppPublisherURL=https://github.com/lolo1580/dcs-mission-manager
AppSupportURL=https://github.com/lolo1580/dcs-mission-manager/issues
DefaultDirName={localappdata}\Programs\DCS Manager
DefaultGroupName=DCS Manager
PrivilegesRequired=lowest
ArchitecturesAllowed=x64compatible
ArchitecturesInstallIn64BitMode=x64compatible
MinVersion=10.0
DisableProgramGroupPage=yes
DisableDirPage=no
DisableWelcomePage=no
WizardStyle=modern dark polar includetitlebar
WizardSizePercent=115,115
WizardBackColor=#111927
WizardImageFile=assets\wizard.bmp
WizardSmallImageFile=assets\header.bmp
WizardImageStretch=no
SetupIconFile=assets\setup.ico
UninstallDisplayIcon={app}\dcsmanager.exe
LicenseFile=..\LICENSE
OutputDir={#OutputPath}
OutputBaseFilename=DCSManager-Setup-{#AppVersion}
Compression=lzma2
SolidCompression=yes
CloseApplications=yes
RestartApplications=no
Uninstallable=yes
#ifdef TestBuild
CreateUninstallRegKey=no
#endif
[Languages]
Name: "french"; MessagesFile: "compiler:Languages\French.isl"
Name: "english"; MessagesFile: "compiler:Default.isl"
[CustomMessages]
french.WelcomeLabel1=Bienvenue dans DCS Manager
french.WelcomeLabel2=Votre compagnon DCS : statistiques, aérodromes et panels.%n%nInstallation pour votre compte Windows, sans droits administrateur. Vos profils restent séparés du programme.
english.WelcomeLabel1=Welcome to DCS Manager
english.WelcomeLabel2=Your DCS companion: statistics, airfields and panels.%n%nInstalls for your Windows account, without administrator rights. Your profiles stay separate from the application.
french.DesktopShortcut=Créer un raccourci sur le Bureau
english.DesktopShortcut=Create a desktop shortcut
french.DCSTitle=Votre installation DCS
english.DCSTitle=Your DCS installation
french.DCSDescription=Vérifiez les dossiers détectés ou corrigez-les.
english.DCSDescription=Check the detected folders or select different ones.
french.DCSHint=Ces dossiers sont facultatifs pour installer l’application. Saved Games est requis pour les scripts Lua. Aucun pilote ni DCS-BIOS n’est installé.
english.DCSHint=These folders are optional for the application. Saved Games is required for Lua scripts. No driver or DCS-BIOS is installed.
french.GameFolder=Dossier du jeu DCS :
english.GameFolder=DCS game folder:
french.SavedGamesFolder=Dossier Saved Games DCS :
english.SavedGamesFolder=DCS Saved Games folder:
french.LuaOption=Installer / mettre à jour les scripts Lua DCS Manager
english.LuaOption=Install / update DCS Manager Lua scripts
french.MigrationTitle=Reprendre la version portable
english.MigrationTitle=Import a portable installation
french.MigrationDescription=Facultatif : copier vos profils et votre historique.
english.MigrationDescription=Optional: copy your profiles and history.
french.MigrationHint=Laissez vide pour une nouvelle installation. Choisissez le dossier contenant la version portable et son sous-dossier data. Les fichiers d’origine restent en place ; les données installées existantes ne seront jamais remplacées. Fermez toutes les instances avant de poursuivre.
english.MigrationHint=Leave blank for a fresh installation. Select the portable application's folder containing its data subfolder. Source files stay intact; existing installed data is never replaced. Close all application instances before proceeding.
french.PortableFolder=Dossier de la version portable :
english.PortableFolder=Portable application folder:
french.InvalidFolder=Vérifiez le dossier choisi. Il doit exister et correspondre à l’installation indiquée.
english.InvalidFolder=Check the selected folder. It must exist and match the indicated installation.
french.CloseApps=Fermez DCS Manager. Si les scripts Lua sont sélectionnés, fermez aussi DCS avant de poursuivre.
english.CloseApps=Close DCS Manager. If Lua scripts are selected, also close DCS before proceeding.
french.SetupFailed=Le programme est installé, mais la préparation des données ou des scripts a échoué. Consultez %1 pour les détails et sauvegardes. Vous pouvez réessayer les scripts dans l’application.
english.SetupFailed=The application is installed, but data or script preparation failed. Check %1 for details and backups. You can retry scripts from the application.
french.DataNotice=Vos données restent dans %1, même après désinstallation. Les scripts DCS ne sont pas retirés automatiquement.
english.DataNotice=Your data stays in %1, including after uninstall. DCS scripts are not automatically removed.
french.LaunchApp=Lancer DCS Manager
english.LaunchApp=Launch DCS Manager
[Tasks]
Name: "desktopicon"; Description: "{cm:DesktopShortcut}"; Flags: unchecked
[Files]
Source: "{#StageDir}\dcsmanager.exe"; DestDir: "{app}"; Flags: ignoreversion
Source: "installed-mode"; DestDir: "{app}"; Flags: ignoreversion
Source: "..\profiles\panels\*.json"; DestDir: "{app}\profiles\panels"; Flags: ignoreversion
Source: "..\docs\plugin-panels-dcs.fr.md"; DestDir: "{app}\docs"; Flags: ignoreversion
Source: "..\docs\installation-windows.fr.md"; DestDir: "{app}\docs"; Flags: ignoreversion
Source: "..\LICENSE"; DestDir: "{app}"; Flags: ignoreversion
[Icons]
#ifndef TestBuild
Name: "{userprograms}\DCS Manager"; Filename: "{app}\dcsmanager.exe"; WorkingDir: "{app}"
Name: "{userdesktop}\DCS Manager"; Filename: "{app}\dcsmanager.exe"; WorkingDir: "{app}"; Tasks: desktopicon
#endif
[Run]
Filename: "{app}\dcsmanager.exe"; Description: "{cm:LaunchApp}"; Flags: nowait postinstall skipifsilent; Check: SetupSucceeded
[Code]
var
  DCSPage, PortablePage: TInputQueryWizardPage;
  LuaOption: TNewCheckBox;
  DataRoot: String;
  ConfigurationOK: Boolean;
function Quote(Value: String): String;
var Index: Integer;
begin
  Result := '"' + Value;
  { Windows command-line parsing requires doubling trailing backslashes. }
  Index := Length(Value);
  while Index > 0 do
  begin
    if Value[Index] <> '\' then Break;
    Result := Result + '\';
    Index := Index - 1;
  end;
  Result := Result + '"';
end;
function SetupSucceeded: Boolean;
begin
  Result := ConfigurationOK;
end;
procedure ChooseFolder(Page: TInputQueryWizardPage; Index: Integer);
var Folder: String;
begin
  Folder := Page.Values[Index];
  if BrowseForFolder(Page.Description, Folder, False) then Page.Values[Index] := Folder;
end;
procedure BrowseGame(Sender: TObject);
begin ChooseFolder(DCSPage, 0); end;
procedure BrowseSavedGames(Sender: TObject);
begin ChooseFolder(DCSPage, 1); end;
procedure BrowsePortable(Sender: TObject);
begin ChooseFolder(PortablePage, 0); end;
procedure AddBrowse(Page: TInputQueryWizardPage; Index: Integer; Handler: TNotifyEvent);
var Button: TNewButton;
begin
  Button := TNewButton.Create(Page);
  Button.Parent := Page.Surface;
  Button.Width := ScaleX(90);
  Button.Height := Page.Edits[Index].Height + ScaleY(2);
  Button.Left := Page.SurfaceWidth - Button.Width;
  Button.Top := Page.Edits[Index].Top;
  Button.Caption := SetupMessage(msgButtonBrowse);
  Button.OnClick := Handler;
  Page.Edits[Index].Width := Button.Left - ScaleX(8);
end;
procedure InitializeWizard;
var
  ResultCode: Integer;
  DetectionFile: String;
begin
  ConfigurationOK := False;
  DataRoot := ExpandConstant('{localappdata}\DCS Manager');
  { Query fields allow empty optional paths; input-dir pages reject them. }
  DCSPage := CreateInputQueryPage(wpSelectTasks, CustomMessage('DCSTitle'),
    CustomMessage('DCSDescription'), CustomMessage('DCSHint'));
  DCSPage.Add(CustomMessage('GameFolder'), False);
  DCSPage.Add(CustomMessage('SavedGamesFolder'), False);
  AddBrowse(DCSPage, 0, @BrowseGame);
  AddBrowse(DCSPage, 1, @BrowseSavedGames);
  LuaOption := TNewCheckBox.Create(DCSPage);
  LuaOption.Parent := DCSPage.Surface;
  LuaOption.Top := DCSPage.Edits[1].Top + DCSPage.Edits[1].Height + ScaleY(20);
  LuaOption.Width := DCSPage.SurfaceWidth;
  LuaOption.Caption := CustomMessage('LuaOption');
  LuaOption.Checked := False;
  PortablePage := CreateInputQueryPage(DCSPage.ID, CustomMessage('MigrationTitle'),
    CustomMessage('MigrationDescription'), CustomMessage('MigrationHint'));
  PortablePage.Add(CustomMessage('PortableFolder'), False);
  AddBrowse(PortablePage, 0, @BrowsePortable);
  ExtractTemporaryFile('dcsmanager.exe');
  DetectionFile := ExpandConstant('{tmp}\dcs-detection.ini');
  if Exec(ExpandConstant('{tmp}\dcsmanager.exe'), 'setup-detect --output ' + Quote(DetectionFile),
    '', SW_HIDE, ewWaitUntilTerminated, ResultCode) then
  begin
    if ResultCode = 0 then
    begin
      DCSPage.Values[0] := GetIniString('DCS', 'Game', '', DetectionFile);
      DCSPage.Values[1] := GetIniString('DCS', 'SavedGames', '', DetectionFile);
    end;
  end;
  WizardForm.WelcomeLabel1.Caption := CustomMessage('WelcomeLabel1');
  WizardForm.WelcomeLabel2.Caption := CustomMessage('WelcomeLabel2');
end;
function NextButtonClick(CurPageID: Integer): Boolean;
begin
  Result := True;
  if CurPageID = DCSPage.ID then
  begin
    if (DCSPage.Values[0] <> '') and not FileExists(AddBackslash(DCSPage.Values[0]) + 'bin\DCS.exe')
      and not FileExists(AddBackslash(DCSPage.Values[0]) + 'bin-mt\DCS.exe') then Result := False;
    if (DCSPage.Values[1] <> '') and not DirExists(DCSPage.Values[1]) then Result := False;
    if LuaOption.Checked and (DCSPage.Values[1] = '') then Result := False;
  end;
  if (CurPageID = PortablePage.ID) and (PortablePage.Values[0] <> '') then
    if not DirExists(AddBackslash(PortablePage.Values[0]) + 'data') then Result := False;
  if not Result then MsgBox(CustomMessage('InvalidFolder'), mbError, MB_OK);
end;
function PrepareToInstall(var NeedsRestart: Boolean): String;
var
  Parameters: String;
  ResultCode: Integer;
begin
  Result := '';
  Parameters := 'setup-check';
  if LuaOption.Checked then Parameters := Parameters + ' --install-lua';
  if not Exec(ExpandConstant('{tmp}\dcsmanager.exe'), Parameters, '', SW_HIDE,
    ewWaitUntilTerminated, ResultCode) then Result := CustomMessage('CloseApps')
  else if ResultCode <> 0 then Result := CustomMessage('CloseApps');
end;
procedure CurStepChanged(CurStep: TSetupStep);
var
  Parameters: String;
  ResultCode: Integer;
begin
  if CurStep = ssPostInstall then
  begin
#ifdef TestBuild
    DataRoot := ExpandConstant('{app}\..\userdata');
    DCSPage.Values[0] := '';
    DCSPage.Values[1] := '';
#endif
    Parameters := 'setup-user --data-root ' + Quote(DataRoot);
    if DCSPage.Values[0] <> '' then Parameters := Parameters + ' --dcs-install ' + Quote(DCSPage.Values[0]);
    if DCSPage.Values[1] <> '' then Parameters := Parameters + ' --saved-games ' + Quote(DCSPage.Values[1]);
    if PortablePage.Values[0] <> '' then Parameters := Parameters + ' --portable-dir ' + Quote(PortablePage.Values[0]);
    if LuaOption.Checked then Parameters := Parameters + ' --install-lua';
    if Exec(ExpandConstant('{app}\dcsmanager.exe'), Parameters, '', SW_HIDE,
      ewWaitUntilTerminated, ResultCode) then ConfigurationOK := ResultCode = 0;
    if not ConfigurationOK then MsgBox(FmtMessage(CustomMessage('SetupFailed'), [DataRoot + '\setup.log']), mbError, MB_OK);
    WizardForm.FinishedLabel.Caption := WizardForm.FinishedLabel.Caption + #13#10#13#10 +
      FmtMessage(CustomMessage('DataNotice'), [DataRoot]);
  end;
end;

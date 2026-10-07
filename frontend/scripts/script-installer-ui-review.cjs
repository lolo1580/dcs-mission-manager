async (page) => {
  page.setDefaultTimeout(5000);
  let status = {savedGames:'C:/Test/Saved Games/DCS',managed:[{destRel:'Scripts/DCSManager/PanelCommands.lua',state:'missing'}],exports:['DCS-BIOS'],thirdParty:[]};
  let requests=0;
  await page.route('**/api/scripts',route=>route.fulfill({contentType:'application/json',body:JSON.stringify(status)}));
  await page.route('**/api/scripts/install',route=>{
    requests++;
    if (requests===1) return route.fulfill({status:409,contentType:'application/json',body:JSON.stringify({error:'Ferme DCS avant d’installer les scripts, puis relance le jeu.'})});
    status={...status,managed:[{destRel:'Scripts/DCSManager/PanelCommands.lua',state:'installed'}]};
    return route.fulfill({contentType:'application/json',body:JSON.stringify({scripts:status,restartRequired:true,results:[{destRel:'Scripts/Export.lua',action:'merged',backup:'C:/Test/Export.lua.bak-test'}]})});
  });
  await page.reload();
  await page.getByRole('button',{name:'Paramètres',exact:true}).click();
  const button=page.getByRole('button',{name:'Installer / mettre à jour les scripts',exact:true});
  await button.waitFor();
  await page.getByText('C:/Test/Saved Games/DCS',{exact:true}).waitFor();
  await button.click();
  await page.getByText('Ferme DCS avant d’installer les scripts, puis relance le jeu.',{exact:true}).waitFor();
  await button.click();
  await page.getByText('Scripts installés. Lance DCS puis charge une mission.',{exact:true}).waitFor();
  await page.getByText('sauvegarde: C:/Test/Export.lua.bak-test',{exact:true}).waitFor();
  await page.getByRole('button',{name:'Rafraîchir',exact:true}).click();
  await page.getByText('à jour',{exact:true}).waitFor();
  status={...status,savedGames:''};
  await page.getByRole('button',{name:'Rafraîchir',exact:true}).click();
  await page.getByText('Non détecté',{exact:true}).waitFor();
  if (await button.isEnabled()) throw new Error('Installer enabled without a detected folder');
  return 'PASS: installer button, running-DCS error, success, backup, status refresh, missing folder';
}

// Playwright CLI setup for UI review. All API calls are intercepted in the
// review browser; the operator's profiles and hardware are never touched.
async (page) => {
  const profiles = {
    Demo_A: { bindings: [{model:'pz70',control:'AP_BUTTON',command:'AP',interface:'action'}, {model:'pz70',control:'LCD_WHEEL',mode:'ALT',command:'ALTITUDE',interface:'variable_step'}], outputs:[], displays:[] },
    Demo_B: { bindings:[], outputs:[], displays:[] },
  };
  const controls = [
    {identifier:'DCSM_PITCH_TRIM',description:'Trim longitudinal — plugin DCS Manager',category:'DCS Manager plugin',inputs:[{interface:'variable_step',suggested_step:1}],outputs:[]},
    {identifier:'AP',description:'Autopilot button',category:'Autopilot',inputs:[{interface:'action'}],outputs:[{type:'integer',address:100,mask:255}]},
    {identifier:'ALTITUDE',description:'Selected altitude',category:'Autopilot',inputs:[{interface:'variable_step',suggested_step:100}],outputs:[{type:'integer',address:102,mask:65535}]},
    {identifier:'HEADING',description:'Selected heading',category:'Autopilot',inputs:[{interface:'variable_step',suggested_step:1}],outputs:[{type:'integer',address:104,mask:65535}]},
    {identifier:'GEAR',description:'Landing gear',category:'Landing gear',inputs:[{interface:'set_state',max_value:1}],outputs:[{type:'integer',address:106,mask:255}]},
  ];
  await page.route('**/api/**', async route => {
    const request = route.request();
    const url = new URL(request.url());
    let body = {};
    if (url.pathname === '/api/aircraft') body = {aircraft:Object.keys(profiles)};
    if (url.pathname === '/api/panels') body = {supported:true,devices:[]};
    if (url.pathname === '/api/panels/plugin') { body={connected:true,aircraft:'FA-18C_hornet',accepted:2}; }
    if (url.pathname === '/api/dcsbios') body = {available:true,connected:false,aircraft:'',frames:0};
    if (url.pathname === '/api/controls') body = {available:true,controls};
    if (url.pathname === '/api/mappings') {
      const aircraft = url.searchParams.get('aircraft');
      if (request.method() === 'POST') profiles[aircraft] = request.postDataJSON();
      body = {profile:{aircraft,...profiles[aircraft]},enabled:false,outputs:false};
    }
    if (url.pathname === '/api/mappings/test') body = {enabled:request.postDataJSON().enabled};
    if (url.pathname === '/api/events') {
      await route.fulfill({status:200,contentType:'text/event-stream',body:': UI review\n\n'});
      return;
    }
    await route.fulfill({status:200,contentType:'application/json',body:JSON.stringify(body)});
  });
  await page.evaluate(() => localStorage.setItem('dcsmanager.lang','fr'));
  await page.reload();
}

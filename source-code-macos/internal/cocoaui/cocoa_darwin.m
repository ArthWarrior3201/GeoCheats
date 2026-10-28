#import <Cocoa/Cocoa.h>
#include <stdlib.h>
#include <string.h>
extern char *gc_go_event(const char *,const char *);

static NSDictionary *event(NSString *name,NSDictionary *input){
    NSData *data=[NSJSONSerialization dataWithJSONObject:input?:@{} options:0 error:nil];
    NSString *json=[[NSString alloc]initWithData:data encoding:NSUTF8StringEncoding];
    char *result=gc_go_event(name.UTF8String,json.UTF8String);
    NSData *output=[NSData dataWithBytes:result length:strlen(result)];free(result);
    return [NSJSONSerialization JSONObjectWithData:output options:0 error:nil]?:@{};
}
static NSTextField *label(NSString *text){return [NSTextField labelWithString:text?:@""];}
static NSButton *button(NSString *text,id target,SEL action){return [NSButton buttonWithTitle:text target:target action:action];}

@interface GCNative : NSObject<NSApplicationDelegate,NSWindowDelegate>
@property(strong) NSWindow *window;
@property(strong) NSStackView *body;
@property(strong) NSTextField *status;
@property(strong) NSDictionary *state;
@property(strong) NSStatusItem *tray;
@property(strong) NSMutableDictionary<NSString*,NSTextField*> *fields;
@property(copy) NSString *name;
@property(copy) NSString *report;
@property(copy) NSString *signature;
@end

@implementation GCNative
- (void)alert:(NSString*)message{NSAlert *a=[NSAlert new];a.alertStyle=NSAlertStyleInformational;a.icon=NSApp.applicationIconImage;a.messageText=message?:@"Could not complete this action";[a runModal];}
- (void)apply:(NSDictionary*)value{
    if(value[@"error"]){[self alert:value[@"error"]];return;}
    self.state=value;if([value[@"refresh"]boolValue])self.signature=nil;[self refresh];
}
- (void)applicationDidFinishLaunching:(NSNotification*)notification{
    NSMenu *menu=[NSMenu new];NSMenuItem *application=[NSMenuItem new];[menu addItem:application];
    NSMenu *appmenu=[[NSMenu alloc]initWithTitle:self.name];application.submenu=appmenu;
    NSMenuItem *change=[[NSMenuItem alloc]initWithTitle:@"Change game…" action:@selector(choose:) keyEquivalent:@""];change.target=self;[appmenu addItem:change];
    if([self.name isEqual:@"GeoCheats"]){NSMenuItem *login=[[NSMenuItem alloc]initWithTitle:@"Start at login" action:@selector(login:) keyEquivalent:@""];login.target=self;login.state=[self.state[@"login"]boolValue]?NSControlStateValueOn:NSControlStateValueOff;[appmenu addItem:login];}
    [appmenu addItem:NSMenuItem.separatorItem];NSMenuItem *quit=[[NSMenuItem alloc]initWithTitle:[@"Quit " stringByAppendingString:self.name] action:@selector(quit:) keyEquivalent:@"q"];quit.target=self;[appmenu addItem:quit];
    NSMenuItem *editRoot=[NSMenuItem new];[menu addItem:editRoot];NSMenu *edit=[[NSMenu alloc]initWithTitle:@"Edit"];editRoot.submenu=edit;
    [edit addItemWithTitle:@"Cut" action:@selector(cut:) keyEquivalent:@"x"];[edit addItemWithTitle:@"Copy" action:@selector(copy:) keyEquivalent:@"c"];[edit addItemWithTitle:@"Paste" action:@selector(paste:) keyEquivalent:@"v"];[edit addItemWithTitle:@"Select All" action:@selector(selectAll:) keyEquivalent:@"a"];
    NSApp.mainMenu=menu;
    self.window=[[NSWindow alloc]initWithContentRect:NSMakeRect(0,0,530,540) styleMask:NSWindowStyleMaskTitled|NSWindowStyleMaskClosable|NSWindowStyleMaskMiniaturizable|NSWindowStyleMaskResizable backing:NSBackingStoreBuffered defer:NO];
    self.window.title=self.name;self.window.delegate=self;self.window.releasedWhenClosed=NO;[self.window center];
    NSStackView *root=[NSStackView stackViewWithViews:@[]];root.orientation=NSUserInterfaceLayoutOrientationVertical;root.alignment=NSLayoutAttributeLeading;root.spacing=18;root.edgeInsets=NSEdgeInsetsMake(20,20,20,20);root.translatesAutoresizingMaskIntoConstraints=NO;
    [self.window.contentView addSubview:root];[NSLayoutConstraint activateConstraints:@[[root.topAnchor constraintEqualToAnchor:self.window.contentView.topAnchor],[root.leadingAnchor constraintEqualToAnchor:self.window.contentView.leadingAnchor],[root.trailingAnchor constraintEqualToAnchor:self.window.contentView.trailingAnchor],[root.bottomAnchor constraintLessThanOrEqualToAnchor:self.window.contentView.bottomAnchor]]];
    NSImageView *icon=[[NSImageView alloc]init];icon.image=NSApp.applicationIconImage;[icon.widthAnchor constraintEqualToConstant:42].active=YES;[icon.heightAnchor constraintEqualToConstant:42].active=YES;
    NSTextField *title=label(self.name);title.font=[NSFont systemFontOfSize:22 weight:NSFontWeightSemibold];NSStackView *header=[NSStackView stackViewWithViews:@[icon,title]];header.spacing=12;[root addArrangedSubview:header];
    self.body=[NSStackView stackViewWithViews:@[]];self.body.orientation=NSUserInterfaceLayoutOrientationVertical;self.body.alignment=NSLayoutAttributeLeading;self.body.spacing=14;[root addArrangedSubview:self.body];[self.body.widthAnchor constraintEqualToAnchor:root.widthAnchor constant:-40].active=YES;
    self.status=label(@"");self.status.lineBreakMode=NSLineBreakByWordWrapping;self.status.maximumNumberOfLines=3;[root addArrangedSubview:self.status];[self.status.widthAnchor constraintEqualToAnchor:self.body.widthAnchor].active=YES;
    [self refresh];
    if([self.name isEqual:@"GeoCheats"]){self.tray=[NSStatusBar.systemStatusBar statusItemWithLength:NSSquareStatusItemLength];NSImage *trayImage=[NSApp.applicationIconImage copy];trayImage.size=NSMakeSize(20,20);self.tray.button.image=trayImage;NSMenu *traymenu=[NSMenu new];NSMenuItem *show=[[NSMenuItem alloc]initWithTitle:@"Open GeoCheats" action:@selector(show:) keyEquivalent:@""];show.target=self;[traymenu addItem:show];NSMenuItem *exit=[[NSMenuItem alloc]initWithTitle:@"Quit" action:@selector(quit:) keyEquivalent:@""];exit.target=self;[traymenu addItem:exit];self.tray.menu=traymenu;}
    BOOL background=[NSProcessInfo.processInfo.arguments containsObject:@"--background"];
    if(!background||self.report.length){[self show:nil];}
    [NSTimer scheduledTimerWithTimeInterval:0.4 target:self selector:@selector(tick:) userInfo:nil repeats:YES];
    if(self.report.length){[NSTimer scheduledTimerWithTimeInterval:1.5 target:self selector:@selector(check:) userInfo:nil repeats:NO];}
}
- (void)refresh{
    NSString *message=self.state[@"message"]?:@"";
    if([message isEqual:@"Game linked."]||[message isEqual:@"Waiting for Geometry Dash."])message=@"";
    self.status.stringValue=message;
    NSData *data=[NSJSONSerialization dataWithJSONObject:@[self.state[@"linked"]?:@NO,self.state[@"mods"]?:@[]] options:0 error:nil];NSString *signature=[[NSString alloc]initWithData:data encoding:NSUTF8StringEncoding];
    if([self.signature isEqual:signature])return;self.signature=signature;
    for(NSView *view in self.body.arrangedSubviews.copy){[self.body removeArrangedSubview:view];[view removeFromSuperview];}
    if(![self.state[@"linked"]boolValue]){[self.body addArrangedSubview:label(@"Link Geometry Dash")];[self.body addArrangedSubview:button(@"Choose file…",self,@selector(choose:))];return;}
    [self.body addArrangedSubview:label(@"Game Linked Successfully")];
    if([self.name isEqual:@"Dash"]){[self renderDash];return;}
    for(NSDictionary *mod in self.state[@"mods"]){
        NSTextField *title=label(mod[@"name"]);[title setContentHuggingPriority:NSLayoutPriorityDefaultLow forOrientation:NSLayoutConstraintOrientationHorizontal];
        NSSwitch *toggle=[NSSwitch new];toggle.identifier=mod[@"id"];toggle.target=self;toggle.action=@selector(toggle:);toggle.state=[mod[@"enabled"]boolValue]?NSControlStateValueOn:NSControlStateValueOff;toggle.enabled=[mod[@"compatible"]boolValue]||[mod[@"enabled"]boolValue];
        NSString *detail=![mod[@"compatible"]boolValue]?@"Unavailable for this build":([mod[@"enabled"]boolValue]?([mod[@"active"]boolValue]?@"Active":@"Waiting for game"):@"");
        NSStackView *row=[NSStackView stackViewWithViews:@[title,label(detail),toggle]];row.spacing=10;row.distribution=NSStackViewDistributionFill;[self.body addArrangedSubview:row];[row.widthAnchor constraintEqualToAnchor:self.body.widthAnchor].active=YES;
    }
    NSStackView *actions=[NSStackView stackViewWithViews:@[button(@"Launch game",self,@selector(launch:)),button(@"Import mod…",self,@selector(importMod:))]];actions.spacing=10;[self.body addArrangedSubview:actions];
}
- (void)renderDash{
    self.fields=[NSMutableDictionary new];NSDictionary *draft=self.state[@"draft"]?:@{};
    NSArray *rows=@[@[@"name",@"Name"],@[@"id",@"Mod ID"],@[@"rva",@"Address (RVA)"],@[@"expected",@"Expected bytes"],@[@"replacement",@"Replacement bytes"]];
    for(NSArray *definition in rows){NSTextField *field=[NSTextField new];field.stringValue=draft[definition[0]]?:@"";self.fields[definition[0]]=field;[field.widthAnchor constraintEqualToConstant:310].active=YES;NSTextField *title=label(definition[1]);[title.widthAnchor constraintEqualToConstant:125].active=YES;NSStackView *row=[NSStackView stackViewWithViews:@[title,field]];row.spacing=10;[self.body addArrangedSubview:row];}
    NSStackView *actions=[NSStackView stackViewWithViews:@[button(@"Create .dash",self,@selector(save:)),button(@"Open .dash…",self,@selector(importMod:)),button(@"Advanced…",self,@selector(advanced:))]];actions.spacing=10;[self.body addArrangedSubview:actions];
}
- (NSMutableDictionary*)draft{NSMutableDictionary *data=[NSMutableDictionary new];for(NSString *key in self.fields)data[key]=self.fields[key].stringValue;return data;}
- (void)choose:(id)sender{NSOpenPanel *panel=[NSOpenPanel openPanel];panel.canChooseFiles=YES;panel.canChooseDirectories=NO;panel.treatsFilePackagesAsDirectories=NO;panel.allowsMultipleSelection=NO;panel.title=@"Choose Geometry Dash";if([panel runModal]==NSModalResponseOK)[self apply:event(@"link",@{@"path":panel.URL.path})];}
- (void)toggle:(NSSwitch*)sender{NSDictionary *updated=event(@"toggle",@{@"id":sender.identifier,@"enabled":@(sender.state==NSControlStateValueOn)});if(updated[@"error"]){sender.state=sender.state==NSControlStateValueOn?NSControlStateValueOff:NSControlStateValueOn;[self alert:updated[@"error"]];return;}[self apply:updated];}
- (void)login:(NSMenuItem*)sender{[self apply:event(@"login",@{@"enabled":@(sender.state!=NSControlStateValueOn)})];sender.state=[self.state[@"login"]boolValue]?NSControlStateValueOn:NSControlStateValueOff;}
- (void)launch:(id)sender{[self apply:event(@"launch",@{})];}
- (void)complete:(id)sender{[self apply:event(@"trigger",@{})];}
- (void)importMod:(id)sender{NSOpenPanel *panel=[NSOpenPanel openPanel];panel.allowedFileTypes=@[@"dash",@"json"];if([panel runModal]!=NSModalResponseOK)return;NSMutableDictionary *input=[self draft];input[@"path"]=panel.URL.path;[self apply:event(@"import",input)];}
- (void)save:(id)sender{
    NSDictionary *input=[self draft];NSDictionary *prepared=event(@"validate",input);if(prepared[@"error"]){[self alert:prepared[@"error"]];return;}
    NSSavePanel *panel=[NSSavePanel savePanel];panel.nameFieldStringValue=prepared[@"filename"]?:@"mod.dash";panel.allowedFileTypes=@[@"dash"];if([panel runModal]!=NSModalResponseOK)return;NSMutableDictionary *request=[input mutableCopy];request[@"path"]=panel.URL.path;[self apply:event(@"save",request)];
}
- (void)advanced:(id)sender{
    NSDictionary *value=event(@"recipe",[self draft]);if(value[@"error"]){[self alert:value[@"error"]];return;}
    NSAlert *alert=[NSAlert new];alert.messageText=@"Recipe JSON";[alert addButtonWithTitle:@"Use recipe"];[alert addButtonWithTitle:@"Cancel"];
    NSScrollView *scroll=[[NSScrollView alloc]initWithFrame:NSMakeRect(0,0,500,330)];scroll.hasVerticalScroller=YES;NSTextView *text=[[NSTextView alloc]initWithFrame:scroll.bounds];text.string=value[@"json"]?:@"";text.font=[NSFont monospacedSystemFontOfSize:12 weight:NSFontWeightRegular];text.richText=NO;scroll.documentView=text;alert.accessoryView=scroll;
    if([alert runModal]==NSAlertFirstButtonReturn)[self apply:event(@"advanced",@{@"json":text.string})];
}
- (void)tick:(id)sender{NSDictionary *state=event(@"state",@{});BOOL show=[state[@"show"]boolValue];[self apply:state];if(show)[self show:nil];}
- (void)show:(id)sender{[self.window makeKeyAndOrderFront:nil];[NSApp activateIgnoringOtherApps:YES];}
- (BOOL)applicationShouldHandleReopen:(NSApplication*)application hasVisibleWindows:(BOOL)visible{[self show:nil];return YES;}
- (BOOL)windowShouldClose:(NSWindow*)window{if([self.name isEqual:@"GeoCheats"]){[window orderOut:nil];return NO;}[self quit:nil];return YES;}
- (void)quit:(id)sender{NSDictionary *result=event(@"quit",@{});if(result[@"error"]){[self alert:result[@"error"]];return;}[NSApp stop:nil];NSEvent *wake=[NSEvent otherEventWithType:NSEventTypeApplicationDefined location:NSZeroPoint modifierFlags:0 timestamp:0 windowNumber:0 context:nil subtype:0 data1:0 data2:0];[NSApp postEvent:wake atStart:YES];}
- (void)check:(id)sender{
    NSView *view=self.window.contentView;[view layoutSubtreeIfNeeded];NSBitmapImageRep *bitmap=[view bitmapImageRepForCachingDisplayInRect:view.bounds];[view cacheDisplayInRect:view.bounds toBitmapImageRep:bitmap];NSData *png=[bitmap representationUsingType:NSBitmapImageFileTypePNG properties:@{}];NSError *error=nil;BOOL captured=[png writeToFile:[self.report stringByAppendingString:@".png"] options:NSDataWritingAtomic error:&error];
    NSDictionary *result=@{@"name":self.name,@"ok":@(captured),@"ui":@"native",@"browser":@NO,@"nativeChooser":@YES,@"choosePromptVisible":@(![self.state[@"linked"]boolValue]),@"checks":@[@"AppKit window",@"native controls",@"native open/save panels",@"app icon",@"rendered capture"]};NSData *data=[NSJSONSerialization dataWithJSONObject:result options:NSJSONWritingPrettyPrinted error:nil];[data writeToFile:self.report atomically:YES];[self quit:nil];
}
@end

static GCNative *application;
void gc_native_run(const char *name,const char *initial,const char *report){@autoreleasepool{
    [NSApplication sharedApplication];[NSApp setActivationPolicy:NSApplicationActivationPolicyRegular];application=[GCNative new];application.name=[NSString stringWithUTF8String:name];application.report=[NSString stringWithUTF8String:report];NSData *data=[[NSString stringWithUTF8String:initial]dataUsingEncoding:NSUTF8StringEncoding];application.state=[NSJSONSerialization JSONObjectWithData:data options:0 error:nil];NSApp.delegate=application;[NSApp run];
}}
